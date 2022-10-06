package kubernetes

import (
	"context"

	"github.com/SchwarzIT/community-stackit-go-client/pkg/api/v1/kubernetes/clusters"
	"github.com/SchwarzIT/community-stackit-go-client/pkg/consts"
	"github.com/SchwarzIT/terraform-provider-stackit/stackit/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	default_retry_duration                 = common.DURATION_20M
	default_allow_privileged               = true
	default_os_name                        = "flatcar"
	default_os_version                     = "3139.2.0"
	default_nodepool_min             int64 = 1
	default_nodepool_max             int64 = 2
	default_nodepool_max_surge       int64 = 1
	default_nodepool_max_unavailable int64 = 1
	default_volume_type                    = "storage_premium_perf1"
	default_volume_size_gb           int64 = 20
	default_cri                            = "containerd"
	default_zone                           = "eu01-m"
)

func (c *Cluster) isHealthyStatus(status string) bool {
	return status == consts.SKE_CLUSTER_STATUS_HEALTHY ||
		status == consts.SKE_CLUSTER_STATUS_HIBERNATED
}

func (c *Cluster) clusterConfig() (clusters.Kubernetes, error) {
	cfg := clusters.Kubernetes{
		Version:                   c.KubernetesVersion.Value,
		AllowPrivilegedContainers: c.AllowPrivilegedContainers.Value,
	}

	if c.AllowPrivilegedContainers.IsNull() || c.AllowPrivilegedContainers.IsUnknown() {
		cfg.AllowPrivilegedContainers = default_allow_privileged
	}
	return cfg, nil
}

func (c *Cluster) nodePools() []clusters.NodePool {
	cnps := []clusters.NodePool{}
	for _, p := range c.NodePools {
		// taints
		ts := []clusters.Taint{}
		for _, v := range p.Taints {
			t := clusters.Taint{
				Effect: v.Effect.Value,
				Key:    v.Key.Value,
				Value:  v.Value.Value,
			}
			ts = append(ts, t)
		}

		// labels
		ls := map[string]string{}
		for k, v := range p.Labels.Elems {
			nv, err := common.ToString(context.Background(), v)
			if err != nil {
				ls[k] = ""
				continue
			}
			ls[k] = nv
		}

		// zones
		zs := []string{}
		for _, v := range p.Zones.Elems {
			if v.IsNull() || v.IsUnknown() {
				continue
			}
			s, err := common.ToString(context.TODO(), v)
			if err != nil {
				continue
			}
			zs = append(zs, s)
		}

		cnp := clusters.NodePool{
			Name:           p.Name.Value,
			Minimum:        int(p.Minimum.Value),
			Maximum:        int(p.Maximum.Value),
			MaxSurge:       int(p.MaxSurge.Value),
			MaxUnavailable: int(p.MaxUnavailable.Value),
			Machine: clusters.Machine{
				Type: p.MachineType.Value,
				Image: clusters.MachineImage{
					Name:    p.OSName.Value,
					Version: p.OSVersion.Value,
				},
			},
			Volume: clusters.Volume{
				Type: p.VolumeType.Value,
				Size: int(p.VolumeSizeGB.Value),
			},
			Taints: ts,
			CRI: clusters.CRI{
				Name: p.ContainerRuntime.Value,
			},
			Labels:            ls,
			AvailabilityZones: zs,
		}
		cnps = append(cnps, cnp)
	}
	return cnps
}

func setNodepoolDefaults(nps []clusters.NodePool) []clusters.NodePool {
	for i, np := range nps {
		if np.Machine.Image.Name == "" {
			nps[i].Machine.Image.Name = default_os_name
		}
		if np.Machine.Image.Version == "" {
			nps[i].Machine.Image.Version = default_os_version
		}
		if np.Minimum == 0 {
			nps[i].Minimum = int(default_nodepool_min)
		}
		if np.Maximum == 0 {
			nps[i].Maximum = int(default_nodepool_max)
		}
		if np.MaxSurge == 0 {
			nps[i].MaxSurge = int(default_nodepool_max_surge)
		}
		if np.MaxUnavailable == 0 {
			nps[i].MaxUnavailable = int(default_nodepool_max_unavailable)
		}
		if np.Volume.Type == "" {
			nps[i].Volume.Type = default_volume_type
		}
		if np.Volume.Size == 0 {
			nps[i].Volume.Size = int(default_volume_size_gb)
		}
		if np.CRI.Name == "" {
			nps[i].CRI.Name = default_cri
		}
		if len(np.AvailabilityZones) == 0 {
			nps[i].AvailabilityZones = []string{default_zone}
		}
	}
	return nps
}

func (c *Cluster) hibernations() *clusters.Hibernation {
	scs := []clusters.HibernationScedule{}
	for _, h := range c.Hibernations {
		sc := clusters.HibernationScedule{
			Start: h.Start.Value,
			End:   h.End.Value,
		}
		if !h.Timezone.IsNull() && !h.Timezone.IsUnknown() {
			sc.Timezone = h.Timezone.Value
		}
		scs = append(scs, sc)
	}

	if len(scs) == 0 {
		return nil
	}

	return &clusters.Hibernation{
		Schedules: scs,
	}
}

func (c *Cluster) extensions() *clusters.Extensions {
	if c.Extensions == nil || c.Extensions.Argus == nil {
		return nil
	}

	return &clusters.Extensions{
		Argus: &clusters.ArgusExtension{
			Enabled:         c.Extensions.Argus.Enabled.Value,
			ArgusInstanceID: c.Extensions.Argus.ArgusInstanceID.Value,
		},
	}
}

func (c *Cluster) maintenance() *clusters.Maintenance {
	if c.Maintenance == nil {
		return nil
	}

	return &clusters.Maintenance{
		AutoUpdate: clusters.MaintenanceAutoUpdate{
			KubernetesVersion:   c.Maintenance.EnableKubernetesVersionUpdates.Value,
			MachineImageVersion: c.Maintenance.EnableMachineImageVersionUpdates.Value,
		},
		TimeWindow: clusters.MaintenanceTimeWindow{
			Start: c.Maintenance.Start.Value,
			End:   c.Maintenance.End.Value,
		},
	}
}

// Transform transforms clusters.Cluster structure to Cluster
func (c *Cluster) Transform(cl clusters.Cluster) {
	c.ID = types.String{Value: cl.Name}
	c.KubernetesVersion = types.String{Value: cl.Kubernetes.Version}
	c.AllowPrivilegedContainers = types.Bool{Value: cl.Kubernetes.AllowPrivilegedContainers}
	c.Status = types.String{Value: cl.Status.Aggregated}

	c.NodePools = []NodePool{}
	for _, np := range cl.Nodepools {
		n := NodePool{
			Name:             types.String{Value: np.Name},
			MachineType:      types.String{Value: np.Machine.Type},
			OSName:           types.String{Value: np.Machine.Image.Name},
			OSVersion:        types.String{Value: np.Machine.Image.Version},
			Minimum:          types.Int64{Value: int64(np.Minimum)},
			Maximum:          types.Int64{Value: int64(np.Maximum)},
			MaxSurge:         types.Int64{Value: int64(np.MaxSurge)},
			MaxUnavailable:   types.Int64{Value: int64(np.MaxUnavailable)},
			VolumeType:       types.String{Value: np.Volume.Type},
			VolumeSizeGB:     types.Int64{Value: int64(np.Volume.Size)},
			Labels:           types.Map{ElemType: types.StringType, Null: true},
			Taints:           nil,
			ContainerRuntime: types.String{Value: np.CRI.Name},
			Zones:            types.List{ElemType: types.StringType, Null: true},
		}
		for k, v := range np.Labels {
			if n.Labels.Null {
				n.Labels.Null = false
				n.Labels.Elems = make(map[string]attr.Value, len(np.Labels))
			}
			n.Labels.Elems[k] = types.String{Value: v}
		}
		for _, v := range np.Taints {
			if n.Taints == nil {
				n.Taints = []Taint{}
			}
			n.Taints = append(n.Taints, Taint{
				Effect: types.String{Value: v.Effect},
				Key:    types.String{Value: v.Key},
				Value:  types.String{Value: v.Value},
			})
		}
		for _, v := range np.AvailabilityZones {
			if n.Zones.Null {
				n.Zones.Null = false
			}
			n.Zones.Elems = append(n.Zones.Elems, types.String{Value: v})
		}
		c.NodePools = append(c.NodePools, n)
	}

	c.transformMaintenance(cl)
	c.transformHibernations(cl)
	c.transformExtensions(cl)
}

func (c *Cluster) transformHibernations(cl clusters.Cluster) {
	if c.Hibernations == nil || cl.Hibernation == nil {
		return
	}

	c.Hibernations = []Hibernation{}
	for _, h := range cl.Hibernation.Schedules {
		c.Hibernations = append(c.Hibernations, Hibernation{
			Start:    types.String{Value: h.Start},
			End:      types.String{Value: h.End},
			Timezone: types.String{Value: h.Timezone},
		})
	}
}

func (c *Cluster) transformMaintenance(cl clusters.Cluster) {
	if c.Maintenance == nil || cl.Maintenance == nil {
		return
	}

	c.Maintenance = &Maintenance{
		EnableKubernetesVersionUpdates:   types.Bool{Value: cl.Maintenance.AutoUpdate.KubernetesVersion},
		EnableMachineImageVersionUpdates: types.Bool{Value: cl.Maintenance.AutoUpdate.MachineImageVersion},
		Start:                            types.String{Value: cl.Maintenance.TimeWindow.Start},
		End:                              types.String{Value: cl.Maintenance.TimeWindow.End},
	}
}

func (c *Cluster) transformExtensions(cl clusters.Cluster) {
	if c.Extensions == nil || cl.Extensions == nil {
		return
	}

	if cl.Extensions.Argus != nil && c.Extensions.Argus != nil {
		c.Extensions.Argus = &ArgusExtension{
			Enabled:         types.Bool{Value: cl.Extensions.Argus.Enabled},
			ArgusInstanceID: types.String{Value: cl.Extensions.Argus.ArgusInstanceID},
		}
	}
}
