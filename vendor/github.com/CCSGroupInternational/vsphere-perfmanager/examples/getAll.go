package main

import (
	"strconv"
	"os"
	"fmt"
	pm "github.com/CCSGroupInternational/vsphere-perfmanager/vspherePerfManager"
	"time"
)

func main() {
	insecure, err := strconv.ParseBool(os.Getenv("VSPHERE_INSECURE"))

	if err != nil {
		fmt.Println("Error to convert VSPHERE_INSECURE env var to bool type\n", err)
	}

	vspherePm := pm.VspherePerfManager{
		Config: pm.Config{
			Vcenter: pm.Vcenter{
				Username: os.Getenv("VSPHERE_USER"),
				Password: os.Getenv("VSPHERE_PASSWORD"),
				Host:     os.Getenv("VSPHERE_HOST"),
				Insecure: insecure,
			},
			Interval: time.Duration(600 * time.Second),
			Data: map[string][]string{
				string(pm.VMs):               {"runtime.host"},
				string(pm.Hosts):             {"parent"},
				string(pm.Clusters):          {},
				string(pm.ResourcePools):     {"parent", "vm"},
				string(pm.Datastores):        {"summary.url", "parent"},
				string(pm.Vapps):             {},
				string(pm.Datacenters):       {},
				string(pm.Folders):           {"parent"},
				string(pm.DatastoreClusters): {"parent"},
			},
			Metrics: map[pm.PmSupportedEntities][]pm.MetricDef{
				pm.Datastores: {
					pm.MetricDef{
						Entities: []string{"datastore1"},
						Metrics:  []string{"disk.unshared.latest"},
					},
				},
				pm.Hosts: {
					pm.MetricDef{
						Metrics: []string{"net.packets*"},
					},
				},
				pm.VMs: {
					pm.MetricDef{
						Metrics:   []string{"net.packets.*"},
						Instances: []string{"vmnic\\d"},
						Entities:  []string{"openshift"},
					},
					pm.MetricDef{
						Metrics:   []string{"disk.*", "mem.*"},
						Entities:  []string{"dropbox"},
					},
				},
			},
			Rollup: pm.Rollup{
				RollupType: []pm.RollupTypes{pm.Maximum, pm.Average},
				Interval: time.Duration(60 * time.Second),
				Metrics: map[pm.PmSupportedEntities][]pm.RollupMetrics{
					pm.VMs: {
						pm.RollupMetrics{
							Metrics:    []string{"net.*"},
							RollupType: []pm.RollupTypes{pm.Latest, pm.Average},
							Interval:   time.Duration(150 * time.Second),
						},
						pm.RollupMetrics{
							Metrics:   []string{"disk.*", "mem.*"},
							RollupType: []pm.RollupTypes{pm.Average},
							Interval:   time.Duration(120 * time.Second),
						},
					},
				},
			},
		},
	}


	err = vspherePm.Init()

	if err != nil {
		fmt.Println("Error on Initializing Vsphere Performance Manager\n", err)
	}

	vms := vspherePm.Get(pm.VMs)

	for _, vm := range vms {
		fmt.Println("VM Name: " + vspherePm.GetProperty(vm, "name").(string))
		host := vspherePm.GetProperty(vm, "runtime.host").(pm.ManagedObject)
		fmt.Println("Host Name :" + vspherePm.GetProperty(host, "name").(string))
		fmt.Println("Cluster Name :" + vspherePm.GetProperty(vspherePm.GetProperty(host, "parent").(pm.ManagedObject), "name").(string))
		for _, metric := range vm.Metrics {
			fmt.Println("Metrics : " + metric.Info.Metric)
			fmt.Println("Metrics Instances: " + metric.Value.Instance)
			fmt.Println("Result: " + strconv.FormatInt(metric.Value.Value, 10))
			fmt.Println( metric.Value.Timestamp )
		}
	}


	hosts := vspherePm.Get(pm.Hosts)

	if err != nil {
		fmt.Println("Error Getting Hosts Metrics\n", err)
	}

	for _, host := range hosts {
		fmt.Println("Host Name: " + vspherePm.GetProperty(host, "name").(string))
		fmt.Println("Cluster Name: " + vspherePm.GetProperty(vspherePm.GetProperty(host, "parent").(pm.ManagedObject),"name").(string))
		for _, metric := range host.Metrics {
			fmt.Println( "Metrics : " + metric.Info.Metric )
			fmt.Println( "Metrics Instances: " + metric.Value.Instance)
			fmt.Println( "Result: " + strconv.FormatInt(metric.Value.Value, 10) )
		}
	}

	dataStores := vspherePm.Get(pm.Datastores)

	for _, dataStore := range dataStores {
		var datastoreCluster, datacenter pm.ManagedObject
		flag := false
		parentObject := vspherePm.GetProperty(dataStore, "parent").(pm.ManagedObject)
		for {
			switch parentType := parentObject.Entity.Type; parentType {
			case string(pm.DatastoreClusters):
				datastoreCluster = parentObject
				parentObject = vspherePm.GetProperty(parentObject, "parent").(pm.ManagedObject)
			case string(pm.Folders):
				parentObject = vspherePm.GetProperty(parentObject, "parent").(pm.ManagedObject)
			case string(pm.Datacenters):
				datacenter = parentObject
				flag = true
			}

			if flag {
				break
			}
		}
		fmt.Println("Datastore Name: " + vspherePm.GetProperty(dataStore, "name").(string))
		if len(datastoreCluster.Entity.Value) != 0 {
			fmt.Println("Datastore Cluster: " + vspherePm.GetProperty(datastoreCluster, "name").(string))
		}
		fmt.Println("Datacenter: " + vspherePm.GetProperty(datacenter, "name").(string))
		fmt.Println("Summary Url: " + vspherePm.GetProperty(dataStore, "summary.url").(string) )
		for _, metric := range dataStore.Metrics {
			fmt.Println( "Metrics : " + metric.Info.Metric )
			var instance string
			if len(metric.Value.Instance) != 0 {
				if _, err := strconv.Atoi(metric.Value.Instance); err == nil {
					instance = vspherePm.GetProperty(vspherePm.GetObject(string(pm.VMs), "vm-" + metric.Value.Instance), "name").(string)
				} else {
					instance = metric.Value.Instance
				}
			} else {
				instance = metric.Value.Instance
			}
			fmt.Println("Metrics Instances: " + instance)
			fmt.Println( "Result: " + strconv.FormatInt(metric.Value.Value, 10) )
		}
	}

	resourcePools := vspherePm.Get(pm.ResourcePools)

	if err != nil {
		fmt.Println("Error Getting ResourcePool Metrics\n", err)
	}

	for _, resourcePool := range resourcePools {
		fmt.Println("Resource Pool: " + vspherePm.GetProperty(resourcePool, "name").(string))
		switch parentType := vspherePm.GetProperty(resourcePool, "parent").(pm.ManagedObject).Entity.Type; parentType {
		case string(pm.Clusters):
			fmt.Println("Cluster Name: " + vspherePm.GetProperty(vspherePm.GetProperty(resourcePool, "parent").(pm.ManagedObject), "name").(string))
		case string(pm.ResourcePools):
			fmt.Println("Cluster Name: " + vspherePm.GetProperty(vspherePm.GetProperty(vspherePm.GetProperty(resourcePool, "parent").(pm.ManagedObject),"parent").(pm.ManagedObject), "name").(string))
		}
		for _, metric := range resourcePool.Metrics {
			fmt.Println( "Metrics : " + metric.Info.Metric )
			fmt.Println( "Metrics Instances: " + metric.Value.Instance)
			fmt.Println( "Result: " + strconv.FormatInt(metric.Value.Value, 10) )
		}
	}

	clusters := vspherePm.Get(pm.Clusters)

	if err != nil {
		fmt.Println("Error Getting ResourcePool Metrics\n", err)
	}

	for _, cluster := range clusters {
		fmt.Println("Cluster Name: " + vspherePm.GetProperty(cluster, "name").(string))
		for _, metric := range cluster.Metrics {
			fmt.Println( "Metrics : " + metric.Info.Metric )
			fmt.Println( "Metrics Instances: " + metric.Value.Instance)
			fmt.Println( "Result: " + strconv.FormatInt(metric.Value.Value, 10) )
		}
	}

	vapps := vspherePm.Get(pm.Vapps)

	if err != nil {
		fmt.Println("Error Getting Vapps Metrics\n", err)
	}

	for _, vapp := range vapps {
		fmt.Println("Vapps Name: " + vspherePm.GetProperty(vapp, "name").(string))
		switch parentType := vspherePm.GetProperty(vapp, "parent").(pm.ManagedObject).Entity.Type; parentType {
			case string(pm.ResourcePools):
				resourcePool := vspherePm.GetProperty(vapp, "parent").(pm.ManagedObject)
				fmt.Println("Resource Pool: " + vspherePm.GetProperty(resourcePool, "name").(string))
				fmt.Println("Cluster Name: " + vspherePm.GetProperty(vspherePm.GetProperty(resourcePool, "parent").(pm.ManagedObject), "name").(string))
		}
		for _, metric := range vapp.Metrics {
			fmt.Println( "Metrics : " + metric.Info.Metric )
			fmt.Println( "Metrics Instances: " + metric.Value.Instance)
			fmt.Println( "Result: " + strconv.FormatInt(metric.Value.Value, 10) )
		}
	}

	datacenters := vspherePm.Get(pm.Datacenters)

	for _, vm := range datacenters {
		fmt.Println("Datacenters Name: " + vspherePm.GetProperty(vm, "name").(string))
		for _, metric := range vm.Metrics {
			fmt.Println("Metrics : " + metric.Info.Metric)
			fmt.Println("Metrics Instances: " + metric.Value.Instance)
			fmt.Println("Result: " + strconv.FormatInt(metric.Value.Value, 10))
		}
	}
}
