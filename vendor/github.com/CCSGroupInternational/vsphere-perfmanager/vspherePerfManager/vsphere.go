package vspherePerfManager

import (
	"context"
	u "github.com/ahl5esoft/golang-underscore"
	"github.com/vmware/govmomi"
	"net/url"
	"strings"
)

type VspherePerfManager struct {
	Config      Config
	client      *govmomi.Client
	metricsInfo map[int32]metricInfo
	objects     map[string]map[string]ManagedObject
	context     context.Context
}

func (v *VspherePerfManager) Init() error {
	err := v.connect(v.Config.Vcenter)
	if err != nil {
		return err
	}
	v.metricsInfo, err = v.getMetricsInfo()
	if err != nil {
		return err
	}

	return v.managedObjects()
}

func (v *VspherePerfManager) connect(c Vcenter) error {
	ctx, cancel := context.WithCancel(context.Background())
	v.context = ctx
	defer cancel()

	u, err := url.Parse(strings.Split(c.Host, "://")[0] + "://" +
		url.QueryEscape(c.Username) + ":" + url.QueryEscape(c.Password) + "@" +
		strings.Split(c.Host, "://")[1] + "/sdk")

	if err != nil {
		return err
	}

	client, err := govmomi.NewClient(ctx, u, c.Insecure)
	if err != nil {
		return err
	}

	v.client = client
	return nil
}

func (v *VspherePerfManager) Disconnect() error {
	return v.client.Logout(v.context)
}

func (v *VspherePerfManager) Get(entityType PmSupportedEntities) []ManagedObject {
	return v.fetch(string(entityType))
}

func (v *VspherePerfManager) fetch(ObjectType string) []ManagedObject {
	var ok bool
	var entities []ManagedObject

	regexs := u.Pluck(v.Config.Metrics[PmSupportedEntities(ObjectType)], "Entities")

	for _, entity := range v.objects[ObjectType] {

		if regexs != nil {
			// Check If entity is to query
			ok = u.Any(regexs.([][]string), func(regex []string, _ int) bool {
				return isMatch(v.GetProperty(entity, "name").(string), regex)
			})

		} else {
			ok = true
		}

		if ok {
			entities = append(entities, v.query(entity))
		}
	}
	return entities
}
