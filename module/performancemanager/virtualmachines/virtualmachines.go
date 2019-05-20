package virtualmachines

import (
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"github.com/CCSGroupInternational/vspherebeat/module/performancemanager"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/cfgwarn"
	"github.com/elastic/beats/metricbeat/mb"
	"github.com/vmware/govmomi/vim25/types"
	"reflect"
	"strings"
	"time"
)

// init registers the MetricSet with the central registry as soon as the program
// starts. The New function will be called later to instantiate an instance of
// the MetricSet for each host defined in the module's configuration. After the
// MetricSet has been created then Fetch will begin to be called periodically.
func init() {
	mb.Registry.MustAddMetricSet("performancemanager", "virtualmachines", New)
}

// MetricSet holds any configuration or state information. It must implement
// the mb.MetricSet interface. And this is best achieved by embedding
// mb.BaseMetricSet because it implements all of the required mb.MetricSet
// interface methods except for Fetch.
type MetricSet struct {
	mb.BaseMetricSet
	Period     time.Duration
	Hosts      []string
	Username   string
	Password   string
	Insecure   bool
	Counters   []interface{}
	Rollup     []interface{}
	MaxQueries int
}


// New creates a new instance of the MetricSet. New is responsible for unpacking
// any MetricSet specific configuration options if there are any.
func New(base mb.BaseMetricSet) (mb.MetricSet, error) {
	cfgwarn.Experimental("The performancemanager virtualmachines metricset is experimental.")

	config := struct{
		Period     time.Duration `config:"period"`
		Hosts      []string      `config:"hosts"`
		Username   string        `config:"username"`
		Password   string        `config:"password"`
		Insecure   bool          `config:"insecure"`
		Counters   []interface{} `config:"counters"`
		Rollup     []interface{} `config:"rollup"`
		MaxQueries int           `config:"maxQueries"`
	}{}
	if err := base.Module().UnpackConfig(&config); err != nil {
		return nil, err
	}

	return &MetricSet{
		BaseMetricSet: base,
		Period:        config.Period,
		Hosts:         config.Hosts,
		Username:      config.Username,
		Password:      config.Password,
		Insecure:      config.Insecure,
		Counters:      config.Counters,
		Rollup:        config.Rollup,
		MaxQueries:    config.MaxQueries,
	}, nil
}

// Fetch methods implements the data gathering and data conversion to the right
// format. It publishes the event which is then forwarded to the output. In case
// of an error set the Error field of mb.Event or simply call report.Error().
func (m *MetricSet) Fetch(report mb.ReporterV2) {
	data := map[string][]string{
		string(pm.VMs):      {
			"runtime.host", "parent", "summary.config.memorySizeMB", "summary.config.guestFullName","summary.config.numCpu",
			"summary.config.numVirtualDisks", "datastore", "config.hardware.device",
		},
		string(pm.Hosts):    {"parent"},
		string(pm.Clusters): {"parent"},
		string(pm.Folders):  {"parent"},
		string(pm.ComputeResources): {"parent"},
		string(pm.Datacenters): {},
		string(pm.Datastores):  {"info", "summary.url"},
	}

	for _, host := range  m.Hosts {
		vspherePm, err := performancemanager.Connect(m.Username, m.Password, host, m.Insecure, m.Period, m.MaxQueries, data)

		if err != nil {
			m.Logger().Panic(err)
			return
		}

		vms := performancemanager.Fetch(m.Name(), m.Counters, m.Rollup, &vspherePm)

		for _, vm := range vms {
			if vm.Error != nil {
				m.Logger().Error(vm.Entity.String() + " => ",  vm.Error)
				continue
			}
			metadata := performancemanager.MetaData(vspherePm, vm)
			host := vspherePm.GetProperty(vm, "runtime.host").(pm.ManagedObject)
			metadataHost := performancemanager.MetaData(vspherePm, host)
			metadataHost["host"] = metadataHost["name"]
			delete(metadataHost, "name")
			delete(metadataHost, "Folder")
			for k, v := range metadataHost {
				metadata[k] = v
			}
			// Provisioned Values
			metadata["Ram"] = common.MapStr{
				"MemorySizeMB": vspherePm.GetProperty(vm, "summary.config.memorySizeMB").(int32),
			}
			metadata["Cpu"] = common.MapStr{
				"NumCpu"      : vspherePm.GetProperty(vm, "summary.config.numCpu").(int32),
			}
			metadata["GuestFullName"] = vspherePm.GetProperty(vm, "summary.config.guestFullName").(string)

			vmfs := make(map[string]string)
			datastores := make(map[string]string)
			for _, datastore := range vspherePm.GetProperty(vm, "datastore").(types.ArrayOfManagedObjectReference).ManagedObjectReference {
				datastoreName := vspherePm.GetProperty(vspherePm.GetObject(string(pm.Datastores), datastore.Value ), "name").(string)
				datastoreUuid := vspherePm.GetProperty(vspherePm.GetObject(string(pm.Datastores), datastore.Value ), "summary.url").(string)
				datastores[strings.Split(datastoreUuid, "/")[len(strings.Split(datastoreUuid, "/"))-2]] = datastoreName
				for _, vmfsInfo := range vspherePm.GetProperty(vspherePm.GetObject(string(pm.Datastores), datastore.Value ), "info").(types.VmfsDatastoreInfo).Vmfs.Extent {
					vmfs[vmfsInfo.DiskName] = datastoreName
				}
			}

			var devices []map[string]interface{}
			var totalCapacityInBytes int64
			for _, device := range vspherePm.GetProperty(vm, "config.hardware.device").(types.ArrayOfVirtualDevice).VirtualDevice {
				switch device.(type) {
				case *types.VirtualDisk:
					devices = append(devices, map[string]interface{}{
						"CapacityInBytes" : device.(*types.VirtualDisk).CapacityInBytes,
						"Name"            : device.(*types.VirtualDisk).DeviceInfo.GetDescription().Label,
						"Datastore"       : vspherePm.GetProperty(vspherePm.GetObject(string(pm.Datastores), reflect.ValueOf(device.(*types.VirtualDisk).Backing).Elem().Interface().(types.VirtualDiskFlatVer2BackingInfo).Datastore.Value ), "name").(string),
					})
					totalCapacityInBytes += device.(*types.VirtualDisk).CapacityInBytes
				}
			}

			metadata["Disks"] = common.MapStr{
				"NumVirtualDisks"      : vspherePm.GetProperty(vm, "summary.config.numVirtualDisks").(int32),
				"TotalCapacityInBytes" : totalCapacityInBytes,
			}

			metadata["Devices"] = make(map[string][]map[string]interface{})
			metadata["Devices"].(map[string][]map[string]interface{})["VirtualDisks"] = make([]map[string]interface{}, len(devices))
			metadata["Devices"].(map[string][]map[string]interface{})["VirtualDisks"] = devices

			for _, metric := range vm.Metrics {
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
