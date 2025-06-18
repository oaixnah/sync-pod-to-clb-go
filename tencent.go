package main

import (
	"encoding/json"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	clb "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/clb/v20180317"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

type TencentClient struct {
	client *clb.Client
	region string
}

type RegisterTarget struct {
	LoadBalancerID string
	ListenerID     string
	LocationID     string
	Port           int
	EniIP          string
}

type DeregisterTarget struct {
	LoadBalancerID string
	ListenerID     string
	LocationID     string
	EniIP          string
	Port           int
}

type Listener struct {
	ListenerID string `json:"ListenerId"`
	Port       int    `json:"Port"`
	Protocol   string `json:"Protocol"`
	Rules      []Rule `json:"Rules"`
}

type Rule struct {
	Domain     string   `json:"Domain"`
	LocationID string   `json:"LocationId"`
	URL        string   `json:"Url"`
	Targets    []Target `json:"Targets"`
}

type Target struct {
	Port               int      `json:"Port"`
	PrivateIPAddresses []string `json:"PrivateIpAddresses"`
}

// type DescribeTargetsResponse struct {
// 	Listeners []Listener `json:"Listeners"`
// }

type DescribeTargetsResponse struct {
	Response struct {
		Listeners []Listener `json:"Listeners"`
		RequestId string     `json:"RequestId"`
	} `json:"Response"`
}

func NewTencentClient() (*TencentClient, error) {
	secretID := os.Getenv("CLOUD_TENCENT_SECRET_ID")
	secretKey := os.Getenv("CLOUD_TENCENT_SECRET_KEY")
	region := os.Getenv("TENCENT_REGION")

	if secretID == "" || secretKey == "" {
		return nil, fmt.Errorf("CLOUD_TENCENT_SECRET_ID and CLOUD_TENCENT_SECRET_KEY must be set")
	}

	if region == "" {
		region = "ap-beijing" // 默认区域
	}

	credential := common.NewCredential(secretID, secretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "clb.tencentcloudapi.com"

	client, err := clb.NewClient(credential, region, cpf)
	if err != nil {
		return nil, fmt.Errorf("failed to create tencent client: %v", err)
	}

	return &TencentClient{
		client: client,
		region: region,
	}, nil
}

func (tc *TencentClient) BatchRegisterTargets(loadBalancerID string, targets []RegisterTarget) error {
	request := clb.NewBatchRegisterTargetsRequest()
	request.LoadBalancerId = common.StringPtr(loadBalancerID)

	var clbTargets []*clb.BatchTarget
	for _, target := range targets {
		clbTarget := &clb.BatchTarget{
			ListenerId: common.StringPtr(target.ListenerID),
			Port:       common.Int64Ptr(int64(target.Port)),
			EniIp:      common.StringPtr(target.EniIP),
		}
		if target.LocationID != "" {
			clbTarget.LocationId = common.StringPtr(target.LocationID)
		}
		clbTargets = append(clbTargets, clbTarget)
	}
	request.Targets = clbTargets

	response, err := tc.client.BatchRegisterTargets(request)
	if _, ok := err.(*errors.TencentCloudSDKError); ok {
		log.Errorf("An API error has returned: %s", err)
		return err
	}
	if err != nil {
		log.Errorf("Failed to register targets: %v", err)
		return err
	}

	log.Debugf("BatchRegisterTargets response: %s", response.ToJsonString())
	return nil
}

func (tc *TencentClient) BatchDeregisterTargets(loadBalancerID string, targets []DeregisterTarget) error {
	request := clb.NewBatchDeregisterTargetsRequest()
	request.LoadBalancerId = common.StringPtr(loadBalancerID)

	var clbTargets []*clb.BatchTarget
	for _, target := range targets {
		clbTarget := &clb.BatchTarget{
			ListenerId: common.StringPtr(target.ListenerID),
			Port:       common.Int64Ptr(int64(target.Port)),
			EniIp:      common.StringPtr(target.EniIP),
		}
		if target.LocationID != "" {
			clbTarget.LocationId = common.StringPtr(target.LocationID)
		}
		clbTargets = append(clbTargets, clbTarget)
	}
	request.Targets = clbTargets

	response, err := tc.client.BatchDeregisterTargets(request)
	if _, ok := err.(*errors.TencentCloudSDKError); ok {
		log.Errorf("An API error has returned: %s", err)
		return err
	}
	if err != nil {
		log.Errorf("Failed to deregister targets: %v", err)
		return err
	}

	log.Debugf("BatchDeregisterTargets response: %s", response.ToJsonString())
	return nil
}

func (tc *TencentClient) DescribeTargets(loadBalancerID string, listenerIDs []string) (*DescribeTargetsResponse, error) {
	request := clb.NewDescribeTargetsRequest()
	request.LoadBalancerId = common.StringPtr(loadBalancerID)

	if len(listenerIDs) > 0 {
		var ids []*string
		for _, id := range listenerIDs {
			ids = append(ids, common.StringPtr(id))
		}
		request.ListenerIds = ids
	}

	response, err := tc.client.DescribeTargets(request)
	if _, ok := err.(*errors.TencentCloudSDKError); ok {
		log.Errorf("An API error has returned: %s", err)
		return nil, err
	}
	if err != nil {
		log.Errorf("Failed to describe targets: %v", err)
		return nil, err
	}

	log.Infof("DescribeTargets response: %s", response.ToJsonString())

	// 解析响应
	var result DescribeTargetsResponse
	err = json.Unmarshal([]byte(response.ToJsonString()), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	return &result, nil
}
