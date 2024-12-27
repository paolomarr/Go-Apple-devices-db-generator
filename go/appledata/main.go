package main

import (
	"crypto/tls"
	"net/http"
	"os"
	"path"

	"appledata/Packages/dbtools"
	"appledata/Packages/wikipedia"

	log "github.com/sirupsen/logrus"
)

func localHTTTPClient() *http.Client {
	tlsconfig := tls.Config{InsecureSkipVerify: true}
	tr := &http.Transport{
		TLSClientConfig: &tlsconfig,
	}
	client := &http.Client{Transport: tr}
	return client
}
func logrusInit() {
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	// LOG_LEVEL not set, let's default to info
	if !ok {
		lvl = "info"
	}
	// parse string, this is built-in feature of logrus
	ll, err := log.ParseLevel(lvl)
	if err != nil {
		ll = log.InfoLevel
	}
	// set global log level
	log.SetLevel(ll)
}

func getDevices() []wikipedia.Device {
	var devices []wikipedia.Device = wikipedia.ParseListOfIphoneModelsTable(localHTTTPClient())
	for _, device := range devices {
		for i := 0; i < len(device.Codenames); i++ {
			cd := device.Codenames[i]
			dbtools.DBAddDevice(device.Modelname, cd, device.Cpu, device.MinOS, device.MaxOS)
		}
	}
	return devices
}

func getVersions() {
	for _, version := range wikipedia.ParseiOSVersionHistory2(localHTTTPClient()) {
		dbtools.DBAddOSVersion(version)
	}
}

type ConfSchema struct {
	Base_url        string
	Processor_pages []string
	Firmware_pages  []string
}

func main() {
	logrusInit()
	cwd, cerr := os.Getwd()
	if cerr != nil {
		log.Fatalf("Unable to get current working directory: %s", cerr.Error())
	}
	dbpath := path.Join(cwd, "build")
	perr := os.MkdirAll(dbpath, os.ModePerm)
	if perr != nil {
		log.Fatalf("Unable to create build directory at path %s: %s", dbpath, perr.Error())
	}
	dbtools.DBInit(dbpath)
	// cpus, err := theiphonewiki.ParseProcessorsPages(localHTTTPClient())
	cpus, err := wikipedia.ParseSystemOnChips(localHTTTPClient())
	if err != nil {
		log.Fatalf("Unable to parse CPUs from theiphonewiki page")
	}
	for _, cpu := range cpus {
		dbtools.DBUpdateCPU(cpu.Code, cpu.Label)
	}
	getVersions()
	getDevices()

	dbtools.DBFlush()
}
