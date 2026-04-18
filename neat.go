package main

import (
	"encoding/json"
)

// cleanResource removes runtime metadata from a resource object
// This implements functionality similar to kubectl-neat
func cleanResource(raw []byte) ([]byte, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}

	// Clean metadata fields
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		// Remove runtime-generated fields
		delete(metadata, "uid")
		delete(metadata, "resourceVersion")
		delete(metadata, "generation")
		delete(metadata, "creationTimestamp")
		delete(metadata, "deletionTimestamp")
		delete(metadata, "deletionGracePeriodSeconds")
		delete(metadata, "selfLink")
		delete(metadata, "managedFields")

		// Clean annotations - remove kubectl and kubernetes system annotations
		if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
			cleanedAnnotations := make(map[string]interface{})
			for key, value := range annotations {
				// Skip system annotations
				if !shouldRemoveAnnotation(key) {
					cleanedAnnotations[key] = value
				}
			}
			if len(cleanedAnnotations) > 0 {
				metadata["annotations"] = cleanedAnnotations
			} else {
				delete(metadata, "annotations")
			}
		}
	}

	// Remove status entirely
	delete(obj, "status")

	// Clean spec fields if present
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		cleanSpec(spec)
	}

	return json.Marshal(obj)
}

// shouldRemoveAnnotation determines if an annotation should be removed
func shouldRemoveAnnotation(key string) bool {
	// List of annotation prefixes to remove
	removePrefix := []string{
		"kubectl.kubernetes.io/",
		"deployment.kubernetes.io/",
		"control-plane.alpha.kubernetes.io/",
		"pv.kubernetes.io/",
		"volume.beta.kubernetes.io/",
		"volume.kubernetes.io/",
		"field.cattle.io/",
		"cattle.io/",
	}

	for _, prefix := range removePrefix {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}

	// Specific annotations to remove
	removeExact := []string{
		"last-applied-configuration",
		"deprecated.daemonset.template.generation",
	}

	for _, exact := range removeExact {
		if key == exact {
			return true
		}
	}

	return false
}

// cleanSpec removes runtime-generated fields from spec
func cleanSpec(spec map[string]interface{}) {
	// For Pods/Deployments/StatefulSets - clean volumeMounts and volumes
	if template, ok := spec["template"].(map[string]interface{}); ok {
		if templateSpec, ok := template["spec"].(map[string]interface{}); ok {
			cleanPodSpec(templateSpec)
		}
	}

	// Clean PVC spec
	delete(spec, "volumeName")
	delete(spec, "volumeMode")

	// Clean Service spec
	if spec["type"] == "ClusterIP" {
		delete(spec, "clusterIP")
		delete(spec, "clusterIPs")
	}
	delete(spec, "sessionAffinity")
}

// cleanPodSpec removes runtime fields from pod spec
func cleanPodSpec(podSpec map[string]interface{}) {
	// Clean containers
	if containers, ok := podSpec["containers"].([]interface{}); ok {
		for _, container := range containers {
			if c, ok := container.(map[string]interface{}); ok {
				delete(c, "terminationMessagePath")
				delete(c, "terminationMessagePolicy")
			}
		}
	}

	// Clean volumes
	if volumes, ok := podSpec["volumes"].([]interface{}); ok {
		for _, volume := range volumes {
			if v, ok := volume.(map[string]interface{}); ok {
				// Remove default service account token volumes
				if secret, ok := v["secret"].(map[string]interface{}); ok {
					if secretName, ok := secret["secretName"].(string); ok {
						// Remove default token volumes
						if len(secretName) > 0 && secretName[:8] == "default-" {
							delete(v, "secret")
						}
					}
				}
			}
		}
	}

	// Remove automatically added fields
	delete(podSpec, "dnsPolicy")
	delete(podSpec, "restartPolicy")
	delete(podSpec, "schedulerName")
	delete(podSpec, "securityContext")
	delete(podSpec, "terminationGracePeriodSeconds")
	delete(podSpec, "serviceAccount")
	delete(podSpec, "serviceAccountName")
}
