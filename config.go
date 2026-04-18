package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	admissionv1 "k8s.io/api/admission/v1"
)

// Config represents the admission controller configuration
type Config struct {
	Watch   WatchConfig   `yaml:"watch"`
	Metrics MetricsConfig `yaml:"metrics"`
}

// WatchConfig defines what resources and namespaces to watch
type WatchConfig struct {
	ClusterWideResources bool            `yaml:"clusterWideResources"`
	Namespaces           NamespaceFilter `yaml:"namespaces"`
	Resources            ResourceFilter  `yaml:"resources"`
	ExcludeUsers         []string        `yaml:"excludeUsers"`
}

// NamespaceFilter defines namespace filtering rules
type NamespaceFilter struct {
	Include        []string          `yaml:"include"`
	Exclude        []string          `yaml:"exclude"`
	IncludePattern []string          `yaml:"includePattern"`
	ExcludePattern []string          `yaml:"excludePattern"`
	LabelSelector  map[string]string `yaml:"labelSelector"`
}

// ResourceFilter defines resource type filtering rules
type ResourceFilter struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// MetricsConfig defines metrics configuration
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

var appConfig *Config

// LoadConfig loads the configuration from file
func LoadConfig(configPath string) (*Config, error) {
	// Default configuration
	config := &Config{
		Watch: WatchConfig{
			ClusterWideResources: false,
			Namespaces: NamespaceFilter{
				Include: []string{},
				Exclude: []string{},
			},
			Resources: ResourceFilter{
				Include: []string{"Deployment", "StatefulSet", "DaemonSet", "Service", "ConfigMap", "Secret", "PersistentVolumeClaim", "Ingress"},
				Exclude: []string{},
			},
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Port:    8443,
			Path:    "/metrics",
		},
	}

	// If config file is provided, load it
	if configPath != "" {
		data, err := ioutil.ReadFile(configPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("⚠️ Config file not found at %s, using defaults", configPath)
				return config, nil
			}
			return nil, err
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, err
		}

		log.Printf("✅ Loaded configuration from %s", configPath)
	} else {
		log.Println("ℹ️ No config file specified, using default configuration")
	}

	return config, nil
}

// ShouldProcessRequest determines if a request should be processed based on configuration
func (c *Config) ShouldProcessRequest(req *admissionv1.AdmissionRequest) bool {
	// Check if user/serviceaccount should be excluded
	if c.shouldExcludeUser(req) {
		log.Printf("⏭️ Skipping request from user %s - user is in exclude list", req.UserInfo.Username)
		return false
	}

	// Check if resource kind should be watched
	if !c.shouldWatchResourceKind(req.Kind.Kind) {
		log.Printf("⏭️ Skipping %s - resource kind not in watch list", req.Kind.Kind)
		return false
	}

	// Check if it's a cluster-wide resource
	if req.Namespace == "" {
		if !c.Watch.ClusterWideResources {
			log.Printf("⏭️ Skipping cluster-wide resource %s - cluster-wide resources not enabled", req.Kind.Kind)
			return false
		}
		return true
	}

	// Check namespace filters
	if !c.shouldWatchNamespace(req.Namespace) {
		log.Printf("⏭️ Skipping %s in namespace %s - namespace not in watch list", req.Kind.Kind, req.Namespace)
		return false
	}

	return true
}

// shouldWatchResourceKind checks if a resource kind should be watched
func (c *Config) shouldWatchResourceKind(kind string) bool {
	// If explicitly excluded, return false
	for _, excluded := range c.Watch.Resources.Exclude {
		if excluded == kind {
			return false
		}
	}

	// If include list is empty, watch all (except excluded)
	if len(c.Watch.Resources.Include) == 0 {
		return true
	}

	// Check if in include list
	for _, included := range c.Watch.Resources.Include {
		if included == kind {
			return true
		}
	}

	return false
}

// shouldWatchNamespace checks if a namespace should be watched
func (c *Config) shouldWatchNamespace(namespace string) bool {
	// Check exact exclude list first
	for _, excluded := range c.Watch.Namespaces.Exclude {
		if excluded == namespace {
			return false
		}
	}

	// Check exclude patterns
	for _, pattern := range c.Watch.Namespaces.ExcludePattern {
		if matchPattern(pattern, namespace) {
			return false
		}
	}

	// If include list is empty and include patterns are empty, watch all (except excluded)
	if len(c.Watch.Namespaces.Include) == 0 && len(c.Watch.Namespaces.IncludePattern) == 0 {
		return true
	}

	// Check exact include list
	for _, included := range c.Watch.Namespaces.Include {
		if included == namespace {
			return true
		}
	}

	// Check include patterns
	for _, pattern := range c.Watch.Namespaces.IncludePattern {
		if matchPattern(pattern, namespace) {
			return true
		}
	}

	return false
}

// IsClusterScopedResource checks if a resource is cluster-scoped
func IsClusterScopedResource(kind string) bool {
	clusterScopedResources := []string{
		"Namespace",
		"PersistentVolume",
		"StorageClass",
		"ClusterRole",
		"ClusterRoleBinding",
		"CustomResourceDefinition",
		"Node",
	}

	for _, resource := range clusterScopedResources {
		if resource == kind {
			return true
		}
	}

	return false
}

// shouldExcludeUser checks if a user/serviceaccount should be excluded
func (c *Config) shouldExcludeUser(req *admissionv1.AdmissionRequest) bool {
	username := req.UserInfo.Username

	// Check against exclude list
	for _, excludedUser := range c.Watch.ExcludeUsers {
		if excludedUser == username {
			return true
		}
	}

	return false
}

// matchPattern matches a namespace/user against a glob pattern
// Supports simple wildcard patterns like "kube-*" or "*-system"
func matchPattern(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		log.Printf("⚠️ Error matching pattern %s: %v", pattern, err)
		return false
	}
	return matched
}
