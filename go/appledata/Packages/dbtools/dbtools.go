package dbtools

import (
	"appledata/Packages/version"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var DB_NAME string = "appledata.sqlite"
var DBRef *gorm.DB

type AppleProcessor struct {
	ID    uint   `gorm:"primaryKey"`
	Code  string `gorm:"unique"`
	Label string
}
type Device struct {
	ID               uint `gorm:"primaryKey"`
	Modelname        string
	Codename         string `gorm:"unique"`
	CpuID            int
	Cpu              AppleProcessor
	OperatingSystems []*OperatingSystem `gorm:"many2many:device_os;"`
}
type OperatingSystem struct {
	ID       uint      `gorm:"primaryKey"`
	Name     string    `gorm:"default:ios"`
	VersionX int       `gorm:"uniqueIndex:unique_version_idx"`
	VersionY int       `gorm:"uniqueIndex:unique_version_idx"`
	VersionZ int       `gorm:"uniqueIndex:unique_version_idx"`
	Models   []*Device `gorm:"many2many:device_os;"`
}
type V_OS_model struct {
	osver_x  string `gorm:"column:osver_x`
	osver_y  string `gorm:"column:osver_y`
	osver_z  string `gorm:"column:osver_z`
	model    string `gorm:"column:model`
	codename string `gorm:"column:codename`
	cpu_abi  string `gorm:"column:cpu_abi`
}

func DBInit(dbbasepath string) {
	var err error

	fullpath := path.Join(dbbasepath, DB_NAME)
	DBRef, err = gorm.Open(sqlite.Open(fullpath), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	// Migrate the schema
	DBRef.AutoMigrate(&AppleProcessor{})
	DBRef.AutoMigrate(&Device{})
	DBRef.AutoMigrate(&OperatingSystem{})

	var earlyCPUs = []AppleProcessor{
		{Code: "S5L8900", Label: "Samsung S5L8900"},
		{Code: "S5L8920", Label: "Samsung S5PC100"},
	}
	DBRef.Create(&earlyCPUs)

	// Wikipedia's iOS10 page does not have the "Version history" section like other pages do
	// We have to initialise iOS10 versions manually
	var ios10versions = []OperatingSystem{
		{Name: "ios", VersionX: 10, VersionY: 0, VersionZ: 1},
		{Name: "ios", VersionX: 10, VersionY: 0, VersionZ: 2},
		{Name: "ios", VersionX: 10, VersionY: 1, VersionZ: 0},
		{Name: "ios", VersionX: 10, VersionY: 1, VersionZ: 1},
		{Name: "ios", VersionX: 10, VersionY: 2, VersionZ: 0},
		{Name: "ios", VersionX: 10, VersionY: 2, VersionZ: 1},
		{Name: "ios", VersionX: 10, VersionY: 3, VersionZ: 0},
		{Name: "ios", VersionX: 10, VersionY: 3, VersionZ: 1},
		{Name: "ios", VersionX: 10, VersionY: 3, VersionZ: 2},
		{Name: "ios", VersionX: 10, VersionY: 3, VersionZ: 3},
		{Name: "ios", VersionX: 10, VersionY: 3, VersionZ: 4},
	}
	DBRef.Create(&ios10versions)

	DBRef.Exec(`DROP VIEW IF EXISTS v_os_model;
	CREATE VIEW v_os_model AS 
	SELECT os.version_x, os.version_y, os.version_z, md.modelname, md.codename, ap.label cpu
	FROM device_os do 
	JOIN devices md ON md.id = do.device_id 
    JOIN apple_processors ap on ap.id = md.cpu_id
	JOIN operating_systems os ON os.id = do.operating_system_id`)
}

func DBFlush() {
	DBRef.Commit()
}

func DBAddDevice(model string, codename string, cpuname string, minos version.OSVersion, maxos version.OSVersion) {
	var device Device
	var cpu AppleProcessor
	DBRef.FirstOrCreate(&device, Device{Codename: codename, Modelname: model})
	DBRef.Save(&device)

	result := DBRef.Where(&AppleProcessor{Label: strings.Replace(cpuname, "Apple ", "", 1)}).First(&cpu)
	if result.RowsAffected == 1 {
		device.Cpu = cpu
		log.Debugf("[DBAddDevice] setting cpu '%s' for device '%s' (%s)", cpuname, model, codename)
		DBRef.Save(&device)
	} else {
		log.Warnf("[DBAddDevice] unknown cpu '%s' for device '%s' (%s)", cpuname, model, codename)
	}
	var osversions []OperatingSystem
	var rng = version.OSVersionRange{
		Left:  version.OSVersionRangeLimit{V: minos, Inclusive: true},
		Right: version.OSVersionRangeLimit{V: maxos, Inclusive: true},
	}
	DBRef.Find(&osversions)
	for _, v := range osversions {
		var osver version.OSVersion = version.OSVersion{X: v.VersionX, Y: v.VersionY, Z: v.VersionZ}
		if osver.InRange(rng) {
			DBRef.Model(&device).Association("OperatingSystems").Append(&v)
		}
	}
}

func DBUpdateCPU(code string, label string) {
	var appproc AppleProcessor
	DBRef.Where(AppleProcessor{Code: code}).Assign(AppleProcessor{Label: label}).FirstOrCreate(&appproc)
	log.Infof("Adding/updating processor %s (%s)", appproc.Label, appproc.Code)
}
func DBAddOSVersion(osVerObject version.OSVersion) {
	var operatingsystem = OperatingSystem{VersionX: osVerObject.X, VersionY: osVerObject.Y, VersionZ: osVerObject.Z}
	DBRef.Clauses(clause.OnConflict{DoNothing: true}).Create(&operatingsystem)
}
