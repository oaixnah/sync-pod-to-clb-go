package main

import (
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type Config struct {
	targets  map[string][]ConfigTarget
	backends map[string][]Backend
	mu       sync.RWMutex
	lastLoad time.Time
	tencent  *TencentClient
}

type ConfigTarget struct {
	LoadBalancerID string
	ListenerID     string
	LocationID     string
	Port           int
}

type Backend struct {
	IP   string
	Port int
}

type RuleConfig struct {
	LoadBalancerID string `yaml:"load_balancer_id"`
	Listeners      []struct {
		Port     int    `yaml:"port"`
		Protocol string `yaml:"protocol"`
		Rules    []struct {
			Domain  string `yaml:"domain"`
			URL     string `yaml:"url"`
			Backend struct {
				Namespace  string `yaml:"namespace"`
				Deployment string `yaml:"deployment"`
				Port       int    `yaml:"port"`
			} `yaml:"backend"`
		} `yaml:"rules"`
	} `yaml:"listeners"`
}

func LoadConfig() (*Config, error) {
	tencent, err := NewTencentClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create tencent client: %v", err)
	}

	config := &Config{
		targets:  make(map[string][]ConfigTarget),
		backends: make(map[string][]Backend),
		tencent:  tencent,
	}

	err = config.loadConfig()
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Config) loadConfig() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查缓存是否过期（60秒）
	if time.Since(c.lastLoad) < 60*time.Second {
		return nil
	}

	// 读取配置文件
	data, err := ioutil.ReadFile("rules.yaml")
	if err != nil {
		return fmt.Errorf("failed to read rules.yaml: %v", err)
	}

	var configs []RuleConfig
	err = yaml.Unmarshal(data, &configs)
	if err != nil {
		return fmt.Errorf("failed to parse rules.yaml: %v", err)
	}

	// 清空旧配置
	c.targets = make(map[string][]ConfigTarget)
	c.backends = make(map[string][]Backend)

	log.Infof("Loaded configs: %v", configs)

	// 处理每个负载均衡器配置
	for _, config := range configs {
		// 获取负载均衡器的监听器列表
		listeners, err := c.getListeners(config.LoadBalancerID)
		log.Infof("listeners: %v", listeners)
		if err != nil {
			log.Errorf("Failed to get listeners for LB %s: %v", config.LoadBalancerID, err)
			continue
		}

		// 匹配配置文件中的转发策略与监听器的转发策略
		for _, configListener := range config.Listeners {
			for _, listener := range listeners {
				if configListener.Port == listener.Port &&
					strings.ToLower(configListener.Protocol) == strings.ToLower(listener.Protocol) {

					for _, configRule := range configListener.Rules {
						for _, rule := range listener.Rules {
							if configRule.Domain == rule.Domain && configRule.URL == rule.URL {
								ns := configRule.Backend.Namespace
								deploy := configRule.Backend.Deployment
								port := configRule.Backend.Port

								// 创建目标配置
								target := ConfigTarget{
									LoadBalancerID: config.LoadBalancerID,
									ListenerID:     listener.ListenerID,
									LocationID:     rule.LocationID,
									Port:           port,
								}

								// 添加到目标列表
								key := fmt.Sprintf("%s/%s", ns, deploy)
								c.targets[key] = append(c.targets[key], target)

								// 创建后端配置
								var backends []Backend
								for _, target := range rule.Targets {
									if len(target.PrivateIPAddresses) > 0 {
										backends = append(backends, Backend{
											IP:   target.PrivateIPAddresses[0],
											Port: target.Port,
										})
									}
								}

								if len(backends) > 0 {
									backendKey := fmt.Sprintf("%s/%s/%s/%s/%s",
										ns, deploy, config.LoadBalancerID, listener.ListenerID, rule.LocationID)
									c.backends[backendKey] = backends
								}
							}
						}
					}
				}
			}
		}
	}

	c.lastLoad = time.Now()
	log.Infof("Config loaded successfully, targets: %d, backends: %d", len(c.targets), len(c.backends))
	return nil
}

func (c *Config) getListeners(loadBalancerID string) ([]Listener, error) {
	response, err := c.tencent.DescribeTargets(loadBalancerID, nil)
	log.Infof("DescribeTargets response: %v", response)
	if err != nil {
		return nil, err
	}

	log.Infof("Listeners: %v", response.Response.Listeners)

	// 只返回 HTTP 和 HTTPS 监听器
	var listeners []Listener
	for _, listener := range response.Response.Listeners {
		if listener.Protocol == "HTTP" || listener.Protocol == "HTTPS" {
			listeners = append(listeners, listener)
		}
	}

	return listeners, nil
}

func (c *Config) GetTargets(key string) []ConfigTarget {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 尝试重新加载配置（如果过期）
	if time.Since(c.lastLoad) >= 60*time.Second {
		c.mu.RUnlock()
		c.loadConfig()
		c.mu.RLock()
	}

	return c.targets[key]
}

func (c *Config) GetBackendIPs(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	backends := c.backends[key]
	var ips []string
	for _, backend := range backends {
		ips = append(ips, backend.IP)
	}
	return ips
}

func (c *Config) GetBackendIPPorts(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	backends := c.backends[key]
	var ipPorts []string
	for _, backend := range backends {
		ipPorts = append(ipPorts, fmt.Sprintf("%s:%d", backend.IP, backend.Port))
	}
	return ipPorts
}

func (c *Config) GetBackendChangePortIPs(key string, targetPort int) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	backends := c.backends[key]
	var ipPorts []string
	for _, backend := range backends {
		if backend.Port != targetPort {
			ipPorts = append(ipPorts, fmt.Sprintf("%s:%d", backend.IP, backend.Port))
		}
	}
	return ipPorts
}
