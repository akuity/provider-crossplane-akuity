//go:build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import ()

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AgentPermissionsRule) DeepCopyInto(out *AgentPermissionsRule) {
	*out = *in
	if in.ApiGroups != nil {
		in, out := &in.ApiGroups, &out.ApiGroups
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Verbs != nil {
		in, out := &in.Verbs, &out.Verbs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AgentPermissionsRule.
func (in *AgentPermissionsRule) DeepCopy() *AgentPermissionsRule {
	if in == nil {
		return nil
	}
	out := new(AgentPermissionsRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppSetDelegate) DeepCopyInto(out *AppSetDelegate) {
	*out = *in
	if in.ManagedCluster != nil {
		in, out := &in.ManagedCluster, &out.ManagedCluster
		*out = new(ManagedCluster)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppSetDelegate.
func (in *AppSetDelegate) DeepCopy() *AppSetDelegate {
	if in == nil {
		return nil
	}
	out := new(AppSetDelegate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AppsetPolicy) DeepCopyInto(out *AppsetPolicy) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AppsetPolicy.
func (in *AppsetPolicy) DeepCopy() *AppsetPolicy {
	if in == nil {
		return nil
	}
	out := new(AppsetPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ArgoCD) DeepCopyInto(out *ArgoCD) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ArgoCD.
func (in *ArgoCD) DeepCopy() *ArgoCD {
	if in == nil {
		return nil
	}
	out := new(ArgoCD)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ArgoCDExtensionInstallEntry) DeepCopyInto(out *ArgoCDExtensionInstallEntry) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ArgoCDExtensionInstallEntry.
func (in *ArgoCDExtensionInstallEntry) DeepCopy() *ArgoCDExtensionInstallEntry {
	if in == nil {
		return nil
	}
	out := new(ArgoCDExtensionInstallEntry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ArgoCDSpec) DeepCopyInto(out *ArgoCDSpec) {
	*out = *in
	in.InstanceSpec.DeepCopyInto(&out.InstanceSpec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ArgoCDSpec.
func (in *ArgoCDSpec) DeepCopy() *ArgoCDSpec {
	if in == nil {
		return nil
	}
	out := new(ArgoCDSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Cluster) DeepCopyInto(out *Cluster) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Cluster.
func (in *Cluster) DeepCopy() *Cluster {
	if in == nil {
		return nil
	}
	out := new(Cluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterCustomization) DeepCopyInto(out *ClusterCustomization) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterCustomization.
func (in *ClusterCustomization) DeepCopy() *ClusterCustomization {
	if in == nil {
		return nil
	}
	out := new(ClusterCustomization)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterData) DeepCopyInto(out *ClusterData) {
	*out = *in
	if in.DatadogAnnotationsEnabled != nil {
		in, out := &in.DatadogAnnotationsEnabled, &out.DatadogAnnotationsEnabled
		*out = new(bool)
		**out = **in
	}
	if in.EksAddonEnabled != nil {
		in, out := &in.EksAddonEnabled, &out.EksAddonEnabled
		*out = new(bool)
		**out = **in
	}
	if in.ManagedClusterConfig != nil {
		in, out := &in.ManagedClusterConfig, &out.ManagedClusterConfig
		*out = new(ManagedClusterConfig)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterData.
func (in *ClusterData) DeepCopy() *ClusterData {
	if in == nil {
		return nil
	}
	out := new(ClusterData)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterSpec) DeepCopyInto(out *ClusterSpec) {
	*out = *in
	in.Data.DeepCopyInto(&out.Data)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterSpec.
func (in *ClusterSpec) DeepCopy() *ClusterSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Command) DeepCopyInto(out *Command) {
	*out = *in
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Command.
func (in *Command) DeepCopy() *Command {
	if in == nil {
		return nil
	}
	out := new(Command)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigManagementPlugin) DeepCopyInto(out *ConfigManagementPlugin) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigManagementPlugin.
func (in *ConfigManagementPlugin) DeepCopy() *ConfigManagementPlugin {
	if in == nil {
		return nil
	}
	out := new(ConfigManagementPlugin)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CrossplaneExtension) DeepCopyInto(out *CrossplaneExtension) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]*CrossplaneExtensionResource, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(CrossplaneExtensionResource)
				**out = **in
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CrossplaneExtension.
func (in *CrossplaneExtension) DeepCopy() *CrossplaneExtension {
	if in == nil {
		return nil
	}
	out := new(CrossplaneExtension)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CrossplaneExtensionResource) DeepCopyInto(out *CrossplaneExtensionResource) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CrossplaneExtensionResource.
func (in *CrossplaneExtensionResource) DeepCopy() *CrossplaneExtensionResource {
	if in == nil {
		return nil
	}
	out := new(CrossplaneExtensionResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Discover) DeepCopyInto(out *Discover) {
	*out = *in
	if in.Find != nil {
		in, out := &in.Find, &out.Find
		*out = new(Find)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Discover.
func (in *Discover) DeepCopy() *Discover {
	if in == nil {
		return nil
	}
	out := new(Discover)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Dynamic) DeepCopyInto(out *Dynamic) {
	*out = *in
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Dynamic.
func (in *Dynamic) DeepCopy() *Dynamic {
	if in == nil {
		return nil
	}
	out := new(Dynamic)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Find) DeepCopyInto(out *Find) {
	*out = *in
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Find.
func (in *Find) DeepCopy() *Find {
	if in == nil {
		return nil
	}
	out := new(Find)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HostAliases) DeepCopyInto(out *HostAliases) {
	*out = *in
	if in.Hostnames != nil {
		in, out := &in.Hostnames, &out.Hostnames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HostAliases.
func (in *HostAliases) DeepCopy() *HostAliases {
	if in == nil {
		return nil
	}
	out := new(HostAliases)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IPAllowListEntry) DeepCopyInto(out *IPAllowListEntry) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IPAllowListEntry.
func (in *IPAllowListEntry) DeepCopy() *IPAllowListEntry {
	if in == nil {
		return nil
	}
	out := new(IPAllowListEntry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageUpdaterDelegate) DeepCopyInto(out *ImageUpdaterDelegate) {
	*out = *in
	if in.ManagedCluster != nil {
		in, out := &in.ManagedCluster, &out.ManagedCluster
		*out = new(ManagedCluster)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageUpdaterDelegate.
func (in *ImageUpdaterDelegate) DeepCopy() *ImageUpdaterDelegate {
	if in == nil {
		return nil
	}
	out := new(ImageUpdaterDelegate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InstanceSpec) DeepCopyInto(out *InstanceSpec) {
	*out = *in
	if in.IpAllowList != nil {
		in, out := &in.IpAllowList, &out.IpAllowList
		*out = make([]*IPAllowListEntry, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(IPAllowListEntry)
				**out = **in
			}
		}
	}
	if in.Extensions != nil {
		in, out := &in.Extensions, &out.Extensions
		*out = make([]*ArgoCDExtensionInstallEntry, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(ArgoCDExtensionInstallEntry)
				**out = **in
			}
		}
	}
	if in.ClusterCustomizationDefaults != nil {
		in, out := &in.ClusterCustomizationDefaults, &out.ClusterCustomizationDefaults
		*out = new(ClusterCustomization)
		**out = **in
	}
	if in.RepoServerDelegate != nil {
		in, out := &in.RepoServerDelegate, &out.RepoServerDelegate
		*out = new(RepoServerDelegate)
		(*in).DeepCopyInto(*out)
	}
	if in.CrossplaneExtension != nil {
		in, out := &in.CrossplaneExtension, &out.CrossplaneExtension
		*out = new(CrossplaneExtension)
		(*in).DeepCopyInto(*out)
	}
	if in.ImageUpdaterDelegate != nil {
		in, out := &in.ImageUpdaterDelegate, &out.ImageUpdaterDelegate
		*out = new(ImageUpdaterDelegate)
		(*in).DeepCopyInto(*out)
	}
	if in.AppSetDelegate != nil {
		in, out := &in.AppSetDelegate, &out.AppSetDelegate
		*out = new(AppSetDelegate)
		(*in).DeepCopyInto(*out)
	}
	if in.AppsetPolicy != nil {
		in, out := &in.AppsetPolicy, &out.AppsetPolicy
		*out = new(AppsetPolicy)
		**out = **in
	}
	if in.HostAliases != nil {
		in, out := &in.HostAliases, &out.HostAliases
		*out = make([]*HostAliases, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(HostAliases)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.AgentPermissionsRules != nil {
		in, out := &in.AgentPermissionsRules, &out.AgentPermissionsRules
		*out = make([]*AgentPermissionsRule, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(AgentPermissionsRule)
				(*in).DeepCopyInto(*out)
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InstanceSpec.
func (in *InstanceSpec) DeepCopy() *InstanceSpec {
	if in == nil {
		return nil
	}
	out := new(InstanceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ManagedCluster) DeepCopyInto(out *ManagedCluster) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ManagedCluster.
func (in *ManagedCluster) DeepCopy() *ManagedCluster {
	if in == nil {
		return nil
	}
	out := new(ManagedCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ManagedClusterConfig) DeepCopyInto(out *ManagedClusterConfig) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ManagedClusterConfig.
func (in *ManagedClusterConfig) DeepCopy() *ManagedClusterConfig {
	if in == nil {
		return nil
	}
	out := new(ManagedClusterConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ParameterAnnouncement) DeepCopyInto(out *ParameterAnnouncement) {
	*out = *in
	if in.Array != nil {
		in, out := &in.Array, &out.Array
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Map != nil {
		in, out := &in.Map, &out.Map
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ParameterAnnouncement.
func (in *ParameterAnnouncement) DeepCopy() *ParameterAnnouncement {
	if in == nil {
		return nil
	}
	out := new(ParameterAnnouncement)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Parameters) DeepCopyInto(out *Parameters) {
	*out = *in
	if in.Static != nil {
		in, out := &in.Static, &out.Static
		*out = make([]*ParameterAnnouncement, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(ParameterAnnouncement)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.Dynamic != nil {
		in, out := &in.Dynamic, &out.Dynamic
		*out = new(Dynamic)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Parameters.
func (in *Parameters) DeepCopy() *Parameters {
	if in == nil {
		return nil
	}
	out := new(Parameters)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PluginSpec) DeepCopyInto(out *PluginSpec) {
	*out = *in
	if in.Init != nil {
		in, out := &in.Init, &out.Init
		*out = new(Command)
		(*in).DeepCopyInto(*out)
	}
	if in.Generate != nil {
		in, out := &in.Generate, &out.Generate
		*out = new(Command)
		(*in).DeepCopyInto(*out)
	}
	if in.Discover != nil {
		in, out := &in.Discover, &out.Discover
		*out = new(Discover)
		(*in).DeepCopyInto(*out)
	}
	if in.Parameters != nil {
		in, out := &in.Parameters, &out.Parameters
		*out = new(Parameters)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PluginSpec.
func (in *PluginSpec) DeepCopy() *PluginSpec {
	if in == nil {
		return nil
	}
	out := new(PluginSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RepoServerDelegate) DeepCopyInto(out *RepoServerDelegate) {
	*out = *in
	if in.ManagedCluster != nil {
		in, out := &in.ManagedCluster, &out.ManagedCluster
		*out = new(ManagedCluster)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RepoServerDelegate.
func (in *RepoServerDelegate) DeepCopy() *RepoServerDelegate {
	if in == nil {
		return nil
	}
	out := new(RepoServerDelegate)
	in.DeepCopyInto(out)
	return out
}
