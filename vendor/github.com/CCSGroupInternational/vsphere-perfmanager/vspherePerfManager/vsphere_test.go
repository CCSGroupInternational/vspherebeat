package vspherePerfManager

import (
	"testing"
	"os"
	"strconv"
	"fmt"
	"github.com/CCSGroupInternational/vsphere-perfmanager/config"
)

func Setup(t *testing.T) (VspherePerfManager){

	insecure, err := strconv.ParseBool(os.Getenv("VSPHERE_INSECURE"))

	if err != nil {
		t.Error("Error to convert VSPHERE_INSECURE env var to bool type\n", err)
	}

	vspherePerfManager := VspherePerfManager{}

	vsphereConfig := config.VsphereConfig {
		Username : os.Getenv("VSPHERE_USER"),
		Password : os.Getenv("VSPHERE_PASSWORD"),
		Host     : os.Getenv("VSPHERE_HOST"),
		Insecure : insecure,
	}

	err = vspherePerfManager.connect(&vsphereConfig)

	if err != nil {
		t.Error("Error connection to VSPHERE\n ", err)
	}

	return vspherePerfManager

}

func TestDataCenters(t *testing.T) {

	vspherePerfManager := Setup(t)

	dataCenters := vspherePerfManager.DataCenters()

	fmt.Println(dataCenters)
}
