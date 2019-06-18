package virtualmachines

import (
	"fmt"
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"github.com/CCSGroupInternational/vspherebeat/module/performancemanager"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/metricbeat/mb"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	"reflect"
	"regexp"
	"strconv"
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

	t1 := time.Now()

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

	vspherePm, err := performancemanager.Connect(m.Usernames[performancemanager.IndexOf(m.Host(), m.Hosts)], m.Passwords[performancemanager.IndexOf(m.Host(), m.Hosts)], m.Host(), m.Insecure, m.Period, m.MaxMetrics, data)

	if err != nil {
		m.Logger().Panic(err)
		return
	}


	t2 := time.Now()
	vms := performancemanager.Fetch(m.Name(), m.Counters, m.Rollup, &vspherePm)
	m.Logger().Info("virtualMachines:collect:" + m.Host() + ":" + time.Now().Sub(t2).String())
	count := 0
	for _, vm := range vms {
		if vm.Error != nil {
			m.Logger().Error(vspherePm.Config.Vcenter.Host + " => " + vm.Entity.String() + " => ",  vm.Error)
			continue
		}
		metadata := performancemanager.MetaData(vspherePm, vm)
		host := vspherePm.GetProperty(vm, "runtime.host")
		if host != nil {
			metadataHost := performancemanager.MetaData(vspherePm, host.(pm.ManagedObject))
			metadataHost["host"] = metadataHost["name"]
			delete(metadataHost, "name")
			delete(metadataHost, "Folder")
			for k, v := range metadataHost {
				metadata[k] = v
			}
		}

		// Provisioned Values
		if memory := vspherePm.GetProperty(vm, "summary.config.memorySizeMB"); memory != nil {
			metadata["Ram"] = common.MapStr{
				"MemorySizeMB": memory.(int32),
			}
		} else {
			metadata["Ram"] = common.MapStr{
				"MemorySizeMB": "",
			}
			m.Logger().Warn(vspherePm.Config.Vcenter.Host + ":" + vm.Entity.String() + ":",  "summary.config.memorySizeMB is nil")
		}

		if cpu := vspherePm.GetProperty(vm, "summary.config.numCpu"); cpu != nil {
			metadata["Cpu"] = common.MapStr{
				"NumCpu": cpu.(int32),
			}
		} else {
			metadata["Cpu"] = common.MapStr{
				"NumCpu": "",
			}
			m.Logger().Warn(vspherePm.Config.Vcenter.Host + ":" + vm.Entity.String() + ":",  "summary.config.numCpu is nil")
		}

		if guestfullname := vspherePm.GetProperty(vm, "summary.config.guestFullName"); guestfullname != nil {
			metadata["GuestFullName"] = guestfullname.(string)
		} else {
			metadata["GuestFullName"] = ""
			m.Logger().Warn(vspherePm.Config.Vcenter.Host + ":" + vm.Entity.String() + ":",  "summary.config.guestFullName is nil")
		}

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
		controllersToDisk := make(map[string]string)
		if hardware := vspherePm.GetProperty(vm, "config.hardware.device"); hardware != nil {
			vmDevices := object.VirtualDeviceList(hardware.(types.ArrayOfVirtualDevice).VirtualDevice)
			for _, device := range hardware.(types.ArrayOfVirtualDevice).VirtualDevice {
				var datastore string
				switch device.(type) {
				case *types.VirtualDisk:

					switch device.(*types.VirtualDisk).Backing.(type) {
					case *types.VirtualDiskSparseVer2BackingInfo:
						datastore = vspherePm.GetProperty(vspherePm.GetObject(string(pm.Datastores), reflect.ValueOf(device.(*types.VirtualDisk).Backing).Elem().Interface().(types.VirtualDiskSparseVer2BackingInfo).Datastore.Value ), "name").(string)
					case *types.VirtualDiskFlatVer2BackingInfo:
						if datastoreObject := reflect.ValueOf(device.(*types.VirtualDisk).Backing).Elem().Interface().(types.VirtualDiskFlatVer2BackingInfo).Datastore; datastoreObject != nil {
							datastore = vspherePm.GetProperty(vspherePm.GetObject(string(pm.Datastores), datastoreObject.Value ), "name").(string)
						}
					case *types.VirtualDiskRawDiskMappingVer1BackingInfo:
						datastore = vspherePm.GetProperty(vspherePm.GetObject(string(pm.Datastores), reflect.ValueOf(device.(*types.VirtualDisk).Backing).Elem().Interface().(types.VirtualDiskRawDiskMappingVer1BackingInfo).Datastore.Value ), "name").(string)
					default:
						datastore = "N/A - Check Beat Logs"
						m.Logger().Warn(vspherePm.Config.Vcenter.Host + ":" + vspherePm.GetProperty(vm, "name").(string) + ":",  "virtualdisk has a different type : " , reflect.ValueOf(device.(*types.VirtualDisk).Backing).Elem().Type() )
					}

					devices = append(devices, map[string]interface{}{
						"CapacityInBytes" : device.(*types.VirtualDisk).CapacityInBytes,
						"Name"            : device.(*types.VirtualDisk).DeviceInfo.GetDescription().Label,
						"Datastore"       : datastore,
					})
					totalCapacityInBytes += device.(*types.VirtualDisk).CapacityInBytes

					if scsi, ok := vmDevices.FindByKey(device.(*types.VirtualDisk).ControllerKey).(types.BaseVirtualSCSIController); ok {
						controllersToDisk[fmt.Sprintf("scsi%d:%d", scsi.GetVirtualSCSIController().BusNumber, *device.(*types.VirtualDisk).UnitNumber)] = device.(*types.VirtualDisk).DeviceInfo.GetDescription().Label
					} else if ide, ok := vmDevices.FindByKey(device.(*types.VirtualDisk).ControllerKey).(*types.VirtualIDEController); ok {
						controllersToDisk[fmt.Sprintf("ide%d:%d", ide.UnitNumber, *device.(*types.VirtualDisk).UnitNumber)] = device.(*types.VirtualDisk).DeviceInfo.GetDescription().Label
					}
				}
			}

			metadata["Disks"] = common.MapStr{
				"NumVirtualDisks"      : vspherePm.GetProperty(vm, "summary.config.numVirtualDisks").(int32),
				"TotalCapacityInBytes" : totalCapacityInBytes,
			}

			metadata["Devices"] = make(map[string][]map[string]interface{})
			metadata["Devices"].(map[string][]map[string]interface{})["VirtualDisks"] = make([]map[string]interface{}, len(devices))
			metadata["Devices"].(map[string][]map[string]interface{})["VirtualDisks"] = devices
		}

		for _, metric := range vm.Metrics {
			instance := metric.Value.Instance
			if len(instance) > 0 {
				if regexp.MustCompile(`^disk\.`).Match([]byte(metric.Info.Metric)) &&
					regexp.MustCompile(`^naa\.`).Match([]byte(instance)) {
					if val, ok := vmfs[instance]; ok {
						instance = val
					}
				} else if regexp.MustCompile(`^datastore\.`).Match([]byte(metric.Info.Metric)) {
					if val, ok := datastores[instance]; ok {
						instance = val
					}
				} else if regexp.MustCompile(`^virtualDisk\.`).Match([]byte(metric.Info.Metric)) {
					if val, ok := controllersToDisk[instance]; ok {
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

			count++
		}
	}

	m.Logger().Info("virtualMachines:finish:" + m.Host() + ":" + time.Now().Sub(t1).String())
	m.Logger().Info("virtualMachines:events:" + m.Host() + ":" + strconv.Itoa(count))
}
