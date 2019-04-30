package vspherePerfManager

import "time"

type Config struct {
	Vcenter
	Interval time.Duration
	Metrics  map[PmSupportedEntities][]MetricDef
	Data     map[string][]string
	Rollup
}

type Vcenter struct {
	Username string
	Password string
	Host     string
	Insecure bool
}

type MetricDef struct {
	Metrics   []string
	Instances []string
	Entities  []string
}

type Rollup struct {
	RollupType   []RollupTypes
	Interval     time.Duration
	Metrics      map[PmSupportedEntities][]RollupMetrics
}

type RollupMetrics struct {
	Metrics    []string
	Instances  []string
	Entities   []string
	RollupType []RollupTypes
	Interval   time.Duration
}

type PmSupportedEntities string
type RollupTypes string

const (
	VMs               PmSupportedEntities = "VirtualMachine"
	Hosts             PmSupportedEntities = "HostSystem"
	ResourcePools     PmSupportedEntities = "ResourcePool"
	Datastores        PmSupportedEntities = "Datastore"
	Clusters          PmSupportedEntities = "ClusterComputeResource"
	Vapps             PmSupportedEntities = "VirtualApp"
	Datacenters       PmSupportedEntities = "Datacenter"
	Folders                               = "Folder"
	DatastoreClusters                     = "StoragePod"
	Average           RollupTypes         = "average"
	Maximum           RollupTypes         = "maximum"
	Minimum           RollupTypes         = "minimum"
	Summation         RollupTypes         = "summation"
	Latest            RollupTypes         = "latest"

)

var ALL = []string{"*"}
