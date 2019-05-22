package hosts

import (
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"github.com/CCSGroupInternational/vspherebeat/module/performancemanager"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/cfgwarn"
	"github.com/elastic/beats/metricbeat/mb"
	"github.com/vmware/govmomi/vim25/types"
	"strings"
	"time"
)

// init registers the MetricSet with the central registry as soon as the program
// starts. The New function will be called later to instantiate an instance of
// the MetricSet for each host defined in the module's configuration. After the
// MetricSet has been created then Fetch will begin to be called periodically.
func init() {
	mb.Registry.MustAddMetricSet("performancemanager", "hosts", New)
}

// MetricSet holds any configuration or state information. It must implement
// the mb.MetricSet interface. And this is best achieved by embedding
// mb.BaseMetricSet because it implements all of the required mb.MetricSet
// interface methods except for Fetch.
type MetricSet struct {
	mb.BaseMetricSet
	Period     time.Duration
	Hosts      []string
	Usernames  []string
	Passwords  []string
	Insecure   bool
	Counters   []interface{}
	Rollup     []interface{}
	MaxMetrics int
}

// New creates a new instance of the MetricSet. New is responsible for unpacking
// any MetricSet specific configuration options if there are any.
func New(base mb.BaseMetricSet) (mb.MetricSet, error) {
	cfgwarn.Experimental("The performancemanager hosts metricset is experimental.")

	config := struct{
		Period     time.Duration `config:"period"`
		Hosts      []string      `config:"hosts"`
		Usernames  []string      `config:"usernames"`
		Passwords  []string      `config:"passwords"`
		Insecure   bool          `config:"insecure"`
		Counters   []interface{} `config:"counters"`
		Rollup     []interface{} `config:"rollup"`
		MaxMetrics int           `config:"maxMetrics"`
	}{}
	if err := base.Module().UnpackConfig(&config); err != nil {
		return nil, err
	}

	return &MetricSet{
		BaseMetricSet: base,
		Period:        config.Period,
		Hosts:         config.Hosts,
		Usernames:     config.Usernames,
		Passwords:     config.Passwords,
		Insecure:      config.Insecure,
		Counters:      config.Counters,
		Rollup:        config.Rollup,
		MaxMetrics:    config.MaxMetrics,
	}, nil
}

// Fetch methods implements the data gathering and data conversion to the right
// format. It publishes the event which is then forwarded to the output. In case
// of an error set the Error field of mb.Event or simply call report.Error().
func (m *MetricSet) Fetch(report mb.ReporterV2) {
	data := map[string][]string{
		string(pm.Hosts)           : {"parent", "datastore", "hardware.cpuInfo.numCpuCores", "hardware.cpuInfo.numCpuThreads", "hardware.cpuInfo.hz" , "hardware.memorySize", "runtime.hostMaxVirtualDiskCapacity", "hardware.systemInfo.vendor"},
		string(pm.Clusters)        : {"parent"},
		string(pm.Folders)         : {"parent"},
		string(pm.ComputeResources): {"parent"},
		string(pm.Datacenters)     : {},
		string(pm.Datastores)      : {"summary.url", "info"},
	}

	for i, host := range  m.Hosts {

		vspherePm, err := performancemanager.Connect(m.Usernames[i], m.Passwords[i], host, m.Insecure, m.Period, m.MaxMetrics, data)
		if err != nil {
			m.Logger().Panic(err)
			return
		}

		hosts := performancemanager.Fetch(m.Name(), m.Counters, m.Rollup, &vspherePm)

		for _, host := range hosts {
			if host.Error != nil {
				m.Logger().Error(host.Entity.String() + " => ",  host.Error)
				continue
			}
			metadata := performancemanager.MetaData(vspherePm, host)

			// Provisioned Values
			metadata["Ram"] = common.MapStr{
				"MemorySize": vspherePm.GetProperty(host, "hardware.memorySize"),
			}
			metadata["Cpu"] = common.MapStr{
				"NumCpuCores"   : vspherePm.GetProperty(host, "hardware.cpuInfo.numCpuCores"),
				"NumCpuThreads" : vspherePm.GetProperty(host, "hardware.cpuInfo.numCpuThreads"),
				"Hz"            : vspherePm.GetProperty(host, "hardware.cpuInfo.hz"),
			}
			metadata["SystemInfo"] = common.MapStr{
				"Vendor": vspherePm.GetProperty(host, "hardware.systemInfo.vendor"),
			}
			metadata["VirtualDisks"] = common.MapStr{
				"HostMaxVirtualDiskCapacity": vspherePm.GetProperty(host, "runtime.hostMaxVirtualDiskCapacity"),
			}

			vmfs := make(map[string]string)
			datastores := make(map[string]string)
			for _, datastore := range vspherePm.GetProperty(host, "datastore").(types.ArrayOfManagedObjectReference).ManagedObjectReference {
				datastoreName := vspherePm.GetProperty(vspherePm.GetObject(string(pm.Datastores), datastore.Value ), "name").(string)
				datastoreUuid := vspherePm.GetProperty(vspherePm.GetObject(string(pm.Datastores), datastore.Value ), "summary.url").(string)
				datastores[strings.Split(datastoreUuid, "/")[len(strings.Split(datastoreUuid, "/"))-2]] = datastoreName
				for _, vmfsInfo := range vspherePm.GetProperty(vspherePm.GetObject(string(pm.Datastores), datastore.Value ), "info").(types.VmfsDatastoreInfo).Vmfs.Extent {
					vmfs[vmfsInfo.DiskName] = datastoreName
				}
			}

			for _, metric := range host.Metrics {
				instance := metric.Value.Instance
				if len(instance) > 0 {
					if strings.Contains(metric.Info.Metric, "disk.") &&
						strings.Contains(instance , "naa.") {
						if val, ok := vmfs[instance]; ok {
							instance = val
						}
					} else if strings.Contains(metric.Info.Metric, "datastore.") {
						if val, ok := datastores[instance]; ok {
							instance = val
						}
					}
				} else {
					instance = "*"
				}

				report.Event(mb.Event{
					MetricSetFields: common.MapStr{
						"metaData" : metadata,
						"metric"   : performancemanager.MetricWithCustomInstance(metric, instance),
					},
				})
			}

		}

	}
}
