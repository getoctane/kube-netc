package cluster

import (
	v1 "k8s.io/api/core/v1"
)

func getZoneLabelValue(labels map[string]string) string {
	zone := labels["topology.kubernetes.io/zone"]
	if zone == "" {
		zone = labels["failure-domain.beta.kubernetes.io/zone"]
	}
	return zone
}

func (c *ClusterInfo) handleNewObject(obj interface{}) {

	var name string
	var kind string
	var namespace string
	var node string
	var zone string

	// var loadBalancerIP string

	var ip string
	var labels map[string]string

	// Copies all of the attributes as the right type
	switch o := obj.(type) {
	case *v1.Pod:
		ip = o.Status.PodIP
		node = o.Spec.NodeName

		name = o.GetName()
		kind = getObjectType(o)
		namespace = o.GetNamespace()

		labels = o.GetLabels()

		zone, _ = c.GetNodeZone(node)

	case *v1.Service:
		ip = o.Spec.ClusterIP

		name = o.GetName()
		kind = getObjectType(o)
		namespace = o.GetNamespace()

		// if o.Spec.Type == "LoadBalancer" && len(o.Status.LoadBalancer.Ingress) > 0 {
		// 	loadBalancerIP = o.Status.LoadBalancer.Ingress[0].IP
		// }

		labels = o.GetLabels()
	case *v1.Node:
		internalIP, err := getNodeIP(o)
		c.check(err)
		ip = internalIP

		name = o.GetName()
		kind = getObjectType(o)
		namespace = o.GetNamespace()

		labels = o.GetLabels()

		zone = getZoneLabelValue(labels)
		c.SetNodeZone(name, zone)
	}

	info := &ObjectInfo{
		Name:      name,
		Kind:      kind,
		Namespace: namespace,
		Node:      node,
		Zone:      zone,
		// LoadBalancerIP: loadBalancerIP,
	}

	info.LabelName = labels["name"]
	info.LabelComponent = labels["component"]
	info.LabelInstance = labels["instance"]
	info.LabelVersion = labels["version"]
	info.LabelPartOf = labels["part-of"]
	info.LabelManagedBy = labels["managed-by"]

	// Updating the map
	c.Logger.Debugw("handling new object map",
		"package", "cluster",
		"kind", kind,
		"ip", ip,
		"name", name,
	)
	c.Set(ip, info)
}

func (c *ClusterInfo) handleUpdateObject(oldObj interface{}, obj interface{}) {
	_ = oldObj

	var name string
	var kind string
	var namespace string
	var node string
	var zone string

	// var loadBalancerIP string

	var ip string
	var labels map[string]string

	// Copies all of the attributes as the right type
	switch o := obj.(type) {
	case *v1.Pod:
		ip = o.Status.PodIP
		node = o.Spec.NodeName

		name = o.GetName()
		kind = getObjectType(o)
		namespace = o.GetNamespace()

		labels = o.GetLabels()

		zone, _ = c.GetNodeZone(node)

	case *v1.Service:
		ip = o.Spec.ClusterIP

		name = o.GetName()
		kind = getObjectType(o)
		namespace = o.GetNamespace()

		// if o.Spec.Type == "LoadBalancer" && len(o.Status.LoadBalancer.Ingress) > 0 {
		// 	loadBalancerIP = o.Status.LoadBalancer.Ingress[0].IP
		// }

		labels = o.GetLabels()
	case *v1.Node:
		internalIP, err := getNodeIP(o)
		c.check(err)
		ip = internalIP

		name = o.GetName()
		kind = getObjectType(o)
		namespace = o.GetNamespace()

		labels = o.GetLabels()

		zone = getZoneLabelValue(labels)
		c.SetNodeZone(name, zone)
	}

	info := &ObjectInfo{
		Name:      name,
		Kind:      kind,
		Namespace: namespace,
		Node:      node,
		Zone:      zone,
		// LoadBalancerIP: loadBalancerIP,
	}

	info.LabelName = labels["name"]
	info.LabelComponent = labels["component"]
	info.LabelInstance = labels["instance"]
	info.LabelVersion = labels["version"]
	info.LabelPartOf = labels["part-of"]
	info.LabelManagedBy = labels["managed-by"]

	c.Logger.Debugw("handling update to object map",
		"package", "cluster",
		"kind", kind,
		"ip", ip,
		"name", name,
	)

	// Updating the map
	c.Set(ip, info)
}

func (c *ClusterInfo) handleDeleteObject(obj interface{}) {

	var ip string

	switch o := obj.(type) {
	case *v1.Pod:
		ip = o.Status.PodIP
	case *v1.Service:
		ip = o.Spec.ClusterIP
	case *v1.Node:
		internalIP, err := getNodeIP(o)
		c.check(err)
		ip = internalIP

		name := o.GetName()
		c.UnsetNodeZone(name)
	}

	c.Logger.Debugw("deleting entry from object map",
		"package", "cluster",
		"ip", ip,
	)

	// Updating the map
	c.Unset(ip)
}
