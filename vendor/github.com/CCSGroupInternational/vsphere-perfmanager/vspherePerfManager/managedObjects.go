package vspherePerfManager

import (
	"github.com/vmware/govmomi/vim25/types"
	"context"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/find"
	u "github.com/ahl5esoft/golang-underscore"
	"reflect"
)

type ManagedObject struct {
	Entity types.ManagedObjectReference
	Properties []types.DynamicProperty
	Metrics []Metric
}

func (v *VspherePerfManager) managedObjects() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var viewManager mo.ViewManager
	err := v.client.RetrieveOne(ctx, *v.client.ServiceContent.ViewManager, nil, &viewManager)
	if err != nil {
		return err
	}

	datacenters, err := v.dataCenters()

	if err != nil {
		return err
	}
	var objectSet []types.ObjectSpec

	keys := reflect.ValueOf(v.Config.Data).MapKeys()
	objectTypes := make([]string, len(keys))
	for i := 0; i < len(keys); i++ {
		objectTypes[i] = keys[i].String()
	}

	for _, datacenter := range datacenters {
		req := types.CreateContainerView{
			This: viewManager.Reference(),
			Container: datacenter,
			Type: objectTypes,
			Recursive: true,
		}

		res, err := methods.CreateContainerView(ctx, v.client.RoundTripper, &req)
		if err != nil {
			return err
		}

		var containerView mo.ContainerView
		err = v.client.RetrieveOne(ctx, res.Returnval, nil, &containerView)
		if err != nil {
			return err
		}

		for _, mor := range containerView.View {
			objectSet = append(objectSet, types.ObjectSpec{Obj: mor, Skip: types.NewBool(false)})
		}

		if _, ok := v.Config.Data[string(Datacenters)]; ok {
			objectSet = append(objectSet, types.ObjectSpec{Obj: datacenter, Skip: types.NewBool(false)})
		}
	}

	return v.retrieveProperties(objectSet, objectTypes)
}

func (v *VspherePerfManager) retrieveProperties(objectSet []types.ObjectSpec, objectTypes []string) (error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	propReq := types.RetrieveProperties{SpecSet: []types.PropertyFilterSpec{{ObjectSet: objectSet, PropSet: setProperties(v.Config.Data)}}}
	propRes, err := v.client.PropertyCollector().RetrieveProperties(ctx, propReq)

	if err != nil {
		return err
	}

	v.objects = make(map[string]map[string]ManagedObject)
	for _, objectType := range objectTypes {
		v.objects[objectType] = make(map[string]ManagedObject)
	}

	for _, objectContent := range propRes.Returnval {
		if _, ok := v.objects[objectContent.Obj.Type]; ok {
			v.objects[objectContent.Obj.Type][objectContent.Obj.Value] = ManagedObject{
				Entity: objectContent.Obj,
				Properties: objectContent.PropSet,
			}
		}
	}

	return nil
}

func (v *VspherePerfManager) dataCenters() ([]types.ManagedObjectReference, error) {

	var dataCenters []types.ManagedObjectReference

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	finder := find.NewFinder(v.client.Client, true)

	dcs, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		return nil, err
	}

	for _, child := range dcs {
		dataCenters = append(dataCenters, child.Reference())
	}

	return dataCenters, nil

}

func (v *VspherePerfManager) GetProperty(o ManagedObject, property string) types.AnyType {
	props := u.Where(o.Properties, func(prop types.DynamicProperty, i int) bool {
		if prop.Name == property {
			return true
		}
		return false
	})

	if props == nil {
		return nil
	}

	switch prop := props.([]types.DynamicProperty)[0].Val.(type) {
		case types.ManagedObjectReference:
			return v.objects[prop.Type][prop.Value]
		default:
			return prop
	}
}

func (v *VspherePerfManager) GetObject(entityType string, objectId string) ManagedObject {
	return v.objects[entityType][objectId]
}

func setProperties(propertiesFromconfig map[string][]string) []types.PropertySpec {
	props := []types.PropertySpec{{
		Type   : "ManagedEntity",
		PathSet : []string{"name"},
		},
	}

	for entity, properties := range propertiesFromconfig {
		props = append(props, types.PropertySpec{
			Type: entity,
			PathSet:properties,
		})
	}
	return props
}