package v1alpha2

// GetObservedGeneration of this Cluster.
func (mg *Cluster) GetObservedGeneration() int64 {
	return mg.Status.GetObservedGeneration()
}

// SetObservedGeneration of this Cluster.
func (mg *Cluster) SetObservedGeneration(generation int64) {
	mg.Status.SetObservedGeneration(generation)
}

// GetObservedGeneration of this Instance.
func (mg *Instance) GetObservedGeneration() int64 {
	return mg.Status.GetObservedGeneration()
}

// SetObservedGeneration of this Instance.
func (mg *Instance) SetObservedGeneration(generation int64) {
	mg.Status.SetObservedGeneration(generation)
}

// GetObservedGeneration of this InstanceIpAllowList.
func (mg *InstanceIpAllowList) GetObservedGeneration() int64 {
	return mg.Status.GetObservedGeneration()
}

// SetObservedGeneration of this InstanceIpAllowList.
func (mg *InstanceIpAllowList) SetObservedGeneration(generation int64) {
	mg.Status.SetObservedGeneration(generation)
}

// GetObservedGeneration of this KargoAgent.
func (mg *KargoAgent) GetObservedGeneration() int64 {
	return mg.Status.GetObservedGeneration()
}

// SetObservedGeneration of this KargoAgent.
func (mg *KargoAgent) SetObservedGeneration(generation int64) {
	mg.Status.SetObservedGeneration(generation)
}

// GetObservedGeneration of this KargoDefaultShardAgent.
func (mg *KargoDefaultShardAgent) GetObservedGeneration() int64 {
	return mg.Status.GetObservedGeneration()
}

// SetObservedGeneration of this KargoDefaultShardAgent.
func (mg *KargoDefaultShardAgent) SetObservedGeneration(generation int64) {
	mg.Status.SetObservedGeneration(generation)
}

// GetObservedGeneration of this KargoInstance.
func (mg *KargoInstance) GetObservedGeneration() int64 {
	return mg.Status.GetObservedGeneration()
}

// SetObservedGeneration of this KargoInstance.
func (mg *KargoInstance) SetObservedGeneration(generation int64) {
	mg.Status.SetObservedGeneration(generation)
}
