package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	// "k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/rest"
)

type PodController struct {
	clientset *kubernetes.Clientset
	tencent   *TencentClient
	config    *Config
}

func NewPodController() (*PodController, error) {
    // 加载 kubeconfig
	// config, err := clientcmd.BuildConfigFromFlags("", "./kube-config")

	// 使用集群内配置
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load in-cluster config: %v", err)
	}

	// 创建 clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %v", err)
	}

	// 创建腾讯云客户端
	tencent, err := NewTencentClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create tencent client: %v", err)
	}

	// 加载配置
	cfg, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	return &PodController{
		clientset: clientset,
		tencent:   tencent,
		config:    cfg,
	}, nil
}

func (pc *PodController) formatLabels(labels map[string]string) string {
	var pairs []string
	for k, v := range labels {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(pairs, ",")
}

func (pc *PodController) getPodIPs(namespace, deploymentName string) ([]string, error) {
	if deploymentName == "" {
		return []string{}, nil
	}

	// 获取 Deployment
	deployment, err := pc.clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %v", namespace, deploymentName, err)
	}

	// 根据 Deployment 的 selector 获取 Pods
	labelSelector := pc.formatLabels(deployment.Spec.Selector.MatchLabels)
	pods, err := pc.clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	var ips []string
	for _, pod := range pods.Items {
		if pod.Status.PodIP != "" {
			ips = append(ips, pod.Status.PodIP)
		}
	}

	return ips, nil
}

func (pc *PodController) getDeploymentName(pod *corev1.Pod) (string, error) {
	if pod.ObjectMeta.OwnerReferences == nil {
		return "", nil
	}

	for _, ownerRef := range pod.ObjectMeta.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" {
			// 获取 ReplicaSet
			rs, err := pc.clientset.AppsV1().ReplicaSets(pod.Namespace).Get(context.TODO(), ownerRef.Name, metav1.GetOptions{})
			if err != nil {
				return "", fmt.Errorf("failed to get replicaset %s: %v", ownerRef.Name, err)
			}

			if rs.ObjectMeta.OwnerReferences != nil {
				for _, rsOwnerRef := range rs.ObjectMeta.OwnerReferences {
					if rsOwnerRef.Kind == "Deployment" {
						return rsOwnerRef.Name, nil
					}
				}
			}
		}
	}

	return "", nil
}

func (pc *PodController) syncPodToLB(namespace, deploymentName, eventType, podName string) error {
	// 获取当前 Pod IPs
	podIPs, err := pc.getPodIPs(namespace, deploymentName)
	if err != nil {
		return fmt.Errorf("failed to get pod IPs: %v", err)
	}

	// 获取配置中的目标
	targets := pc.config.GetTargets(fmt.Sprintf("%s/%s", namespace, deploymentName))
	if len(targets) == 0 {
		return nil // 没有配置，跳过
	}

	for _, target := range targets {
		loadBalancerID := target.LoadBalancerID
		backendKey := fmt.Sprintf("%s/%s/%s/%s/%s", namespace, deploymentName, loadBalancerID, target.ListenerID, target.LocationID)

		// 获取当前后端 IPs
		backendIPs := pc.config.GetBackendIPs(backendKey)

		// 添加新 IP
		newIPs := difference(podIPs, backendIPs)
		if len(newIPs) > 0 {
			log.Infof("%s %s %s %s %s %s Adding new backend: %v",
				time.Now().Format("2006-01-02T15:04:05"),
				namespace, deploymentName, eventType, podName, loadBalancerID, newIPs)

			var registerTargets []RegisterTarget
			for _, ip := range newIPs {
				registerTargets = append(registerTargets, RegisterTarget{
					LoadBalancerID: target.LoadBalancerID,
					ListenerID:     target.ListenerID,
					LocationID:     target.LocationID,
					Port:           target.Port,
					EniIP:          ip,
				})
			}

			err := pc.tencent.BatchRegisterTargets(loadBalancerID, registerTargets)
			if err != nil {
				log.Errorf("Failed to register targets: %v", err)
			}
		}

		// 删除旧 IP
		backendIPPorts := pc.config.GetBackendIPPorts(backendKey)
		backendChangePortIPs := pc.config.GetBackendChangePortIPs(backendKey, target.Port)
		podIPPorts := make([]string, len(podIPs))
		for i, ip := range podIPs {
			podIPPorts[i] = fmt.Sprintf("%s:%d", ip, target.Port)
		}

		oldIPs := intersection(difference(backendIPPorts, podIPPorts), backendChangePortIPs)
		if len(oldIPs) > 0 {
			log.Infof("%s %s %s %s %s %s Removing old backend: %v",
				time.Now().Format("2006-01-02T15:04:05"),
				namespace, deploymentName, eventType, podName, loadBalancerID, oldIPs)

			var deregisterTargets []DeregisterTarget
			for _, ipPort := range oldIPs {
				parts := strings.Split(ipPort, ":")
				if len(parts) == 2 {
					port := 0
					fmt.Sscanf(parts[1], "%d", &port)
					deregisterTargets = append(deregisterTargets, DeregisterTarget{
						LoadBalancerID: target.LoadBalancerID,
						ListenerID:     target.ListenerID,
						LocationID:     target.LocationID,
						EniIP:          parts[0],
						Port:           port,
					})
				}
			}

			err := pc.tencent.BatchDeregisterTargets(loadBalancerID, deregisterTargets)
			if err != nil {
				log.Errorf("Failed to deregister targets: %v", err)
			}
		}
	}

	return nil
}

func (pc *PodController) watchPods(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 创建 watcher
		watcher, err := pc.clientset.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{
			FieldSelector:  fields.Everything().String(),
			TimeoutSeconds: func() *int64 { i := int64(3600); return &i }(),
		})
		if err != nil {
			log.Errorf("Failed to create watcher: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Info("Start watching pod events...")

		for event := range watcher.ResultChan() {
		    // ctrl + c 时，退出循环
			select {
			case <-ctx.Done():
				watcher.Stop()
				return ctx.Err()
			default:
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			namespace := pod.Namespace
			podName := pod.Name
			eventType := string(event.Type)

			deploymentName, err := pc.getDeploymentName(pod)
			if err != nil {
				log.Errorf("Failed to get deployment name for pod %s/%s: %v", namespace, podName, err)
				continue
			}

			if deploymentName == "" {
				continue // 跳过没有 deployment 的 pod
			}

			err = pc.syncPodToLB(namespace, deploymentName, eventType, podName)
			if err != nil {
				log.Errorf("Failed to sync pod to LB: %v", err)
			}

			// log.Infof("===")
		}

		watcher.Stop()
		log.Warning("Watcher stopped, retrying...")
		time.Sleep(5 * time.Second)
	}
}

func (pc *PodController) Run(ctx context.Context) error {
	return pc.watchPods(ctx)
}

// 辅助函数：计算两个字符串切片的差集
func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

// 辅助函数：计算两个字符串切片的交集
func intersection(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var inter []string
	for _, x := range a {
		if _, found := mb[x]; found {
			inter = append(inter, x)
		}
	}
	return inter
}

func main() {
	// 设置日志格式
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   true,
	})
	log.SetLevel(log.InfoLevel)

	// 创建控制器
	controller, err := NewPodController()
	if err != nil {
		log.Fatalf("Failed to create pod controller: %v", err)
	}

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Info("Received shutdown signal")
		cancel()
	}()

	// 运行控制器
	log.Info("Starting pod controller...")
	err = controller.Run(ctx)
	if err != nil && err != context.Canceled {
		log.Fatalf("Controller error: %v", err)
	}

	log.Info("Pod controller stopped")
}
