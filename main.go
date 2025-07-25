package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lightsail"
	"github.com/gin-gonic/gin"
)

// Config 配置结构
type Config struct {
	Instances []InstanceConfig `json:"instances"`
}

// InstanceConfig 实例配置
type InstanceConfig struct {
	Name         string `json:"name"`
	InstanceName string `json:"instance_name"`
	StaticIPName string `json:"static_ip_name"`
}

// IPChangeResult IP切换结果
type IPChangeResult struct {
	InstanceName string `json:"instance_name"`
	OldIP        string `json:"old_ip"`
	NewIP        string `json:"new_ip"`
	Message      string `json:"message"`
	Success      bool   `json:"success"`
}

// Global variables
var (
	config *Config
	svc    *lightsail.Lightsail
)

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var cfg Config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &cfg, nil
}

func changeIP(svc *lightsail.Lightsail, instanceName, staticIPName string) (string, string, error) {
	oldIP := "0.0.0.0"
	newIP := ""

	// 获取实例信息
	instanceOutput, err := svc.GetInstance(&lightsail.GetInstanceInput{
		InstanceName: aws.String(instanceName),
	})
	if err != nil {
		return oldIP, newIP, fmt.Errorf("failed to get instance: %v", err)
	}

	inst := instanceOutput.Instance
	oldIP = *inst.PublicIpAddress

	// 如果已经绑定了静态 IP，就先解绑并释放
	if *inst.IsStaticIp {
		_, _ = svc.DetachStaticIp(&lightsail.DetachStaticIpInput{
			StaticIpName: aws.String(staticIPName),
		})
		_, _ = svc.ReleaseStaticIp(&lightsail.ReleaseStaticIpInput{
			StaticIpName: aws.String(staticIPName),
		})

		// 等待一段时间让 IP 更新
		time.Sleep(5 * time.Second)
		instanceOutput, _ := svc.GetInstance(&lightsail.GetInstanceInput{
			InstanceName: aws.String(instanceName),
		})
		newIP = *instanceOutput.Instance.PublicIpAddress
		log.Printf("Detached and released old static IP. Old IP: %s, New IP: %s", oldIP, newIP)
		return oldIP, newIP, nil
	}

	// 尝试获取旧静态 IP，如果存在则解绑并释放
	_, err = svc.GetStaticIp(&lightsail.GetStaticIpInput{
		StaticIpName: aws.String(staticIPName),
	})
	if err == nil {
		// 存在则尝试解绑
		_, _ = svc.DetachStaticIp(&lightsail.DetachStaticIpInput{
			StaticIpName: aws.String(staticIPName),
		})
		// 释放
		_, _ = svc.ReleaseStaticIp(&lightsail.ReleaseStaticIpInput{
			StaticIpName: aws.String(staticIPName),
		})
		log.Printf("Detached and released existing static IP: %s", staticIPName)
	}

	// 分配新的静态 IP
	_, err = svc.AllocateStaticIp(&lightsail.AllocateStaticIpInput{
		StaticIpName: aws.String(staticIPName),
	})
	if err != nil {
		return oldIP, newIP, fmt.Errorf("failed to allocate static ip: %v", err)
	}
	log.Printf("Allocated new static IP: %s", staticIPName)

	// 绑定到实例
	_, err = svc.AttachStaticIp(&lightsail.AttachStaticIpInput{
		StaticIpName: aws.String(staticIPName),
		InstanceName: aws.String(instanceName),
	})
	if err != nil {
		return oldIP, newIP, fmt.Errorf("failed to attach static ip: %v", err)
	}
	log.Printf("Attached static IP %s to instance %s", staticIPName, instanceName)

	// 获取新 IP
	time.Sleep(5 * time.Second)
	instanceOutput, _ = svc.GetInstance(&lightsail.GetInstanceInput{
		InstanceName: aws.String(instanceName),
	})
	newIP = *instanceOutput.Instance.PublicIpAddress

	return oldIP, newIP, nil
}

// changeIPHandler 处理 IP 切换请求
func changeIPHandler(c *gin.Context) {
	instanceName := c.Query("instance")
	if instanceName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "instance parameter is required",
		})
		return
	}

	// 查找配置中的实例
	var targetInstance *InstanceConfig
	for _, inst := range config.Instances {
		if inst.Name == instanceName {
			targetInstance = &inst
			break
		}
	}

	if targetInstance == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("instance '%s' not found in config", instanceName),
		})
		return
	}

	log.Printf("Changing IP for instance: %s", targetInstance.InstanceName)

	oldIP, newIP, err := changeIP(svc, targetInstance.InstanceName, targetInstance.StaticIPName)
	if err != nil {
		log.Printf("changeIP failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":  false,
			"message":  fmt.Sprintf("changeIP failed: %v", err),
			"instance": targetInstance.InstanceName,
			"old_ip":   oldIP,
			"new_ip":   newIP,
		})
		return
	}

	result := IPChangeResult{
		InstanceName: targetInstance.InstanceName,
		OldIP:        oldIP,
		NewIP:        newIP,
		Message:      "IP changed successfully",
		Success:      true,
	}

	log.Printf("IP changed successfully - Instance: %s, Old IP: %s, New IP: %s",
		targetInstance.InstanceName, oldIP, newIP)

	c.JSON(http.StatusOK, result)
}

// listInstancesHandler 列出所有可管理的实例
func listInstancesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config.Instances,
	})
}

// healthHandler 健康检查
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Service is running",
	})
}

func main() {
	// 加载配置文件
	var err error
	config, err = loadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 从环境变量获取区域，如果没有设置则使用默认值
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1" // 默认区域
	}

	// 创建 session（会自动从环境变量获取凭证）
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}

	svc = lightsail.New(sess)

	log.Printf("Starting IP rotation service, region: %s", region)
	log.Printf("Available instances: %d", len(config.Instances))

	// 设置 Gin 为发布模式
	gin.SetMode(gin.ReleaseMode)

	// 创建路由
	r := gin.Default()

	// API 路由
	r.GET("/health", healthHandler)
	r.GET("/instances", listInstancesHandler)
	r.GET("/change-ip", changeIPHandler)

	// 启动服务
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(r.Run(":" + port))
}
