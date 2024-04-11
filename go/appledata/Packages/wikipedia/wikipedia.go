package wikipedia

import (
	"appledata/Packages/version"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dlclark/regexp2"
	htmltable "github.com/nfx/go-htmltable"
	log "github.com/sirupsen/logrus"
)

var ListOfIphoneModelsURL string = "https://en.wikipedia.org/wiki/List_of_iPhone_models"
var IOSVersionHistoryURL string = "https://en.wikipedia.org/wiki/IOS_version_history"
var IOSVersionPages []string = []string{
	"https://en.wikipedia.org/wiki/IPhone_OS_2",
	"https://en.wikipedia.org/wiki/IPhone_OS_3",
	"https://en.wikipedia.org/wiki/IPhone_OS_4",
	"https://en.wikipedia.org/wiki/IPhone_OS_5",
	"https://en.wikipedia.org/wiki/IPhone_OS_6",
	"https://en.wikipedia.org/wiki/IPhone_OS_7",
	"https://en.wikipedia.org/wiki/IPhone_OS_8",
	"https://en.wikipedia.org/wiki/IPhone_OS_9",
	"https://en.wikipedia.org/wiki/IPhone_OS_10",
	"https://en.wikipedia.org/wiki/IPhone_OS_11",
	"https://en.wikipedia.org/wiki/IPhone_OS_12",
	"https://en.wikipedia.org/wiki/IPhone_OS_13",
	"https://en.wikipedia.org/wiki/IPhone_OS_14",
	"https://en.wikipedia.org/wiki/IPhone_OS_15",
	"https://en.wikipedia.org/wiki/IPhone_OS_16",
	"https://en.wikipedia.org/wiki/IOS_17",
}

var IOSSystemOnChipsPage string = "https://en.wikipedia.org/wiki/List_of_iPhone_models#iPhone_systems-on-chips"

type TableCPU struct {
	Label            string `header:"System-on-chip"`
	Ram              string `header:"RAM"`
	RamType          string `header:"RAM Type"`
	StorageType      string `header:"Storage Type"`
	ModelName        string `header:"Model"`
	LatestIosVersion string `header:"Highest Supported iOS"`
}
type Device struct {
	Codenames []string
	Cpu       string
	Modelname string
	MinOS     version.OSVersion
	MaxOS     version.OSVersion
}
type Cpu struct {
	Code  string
	Label string
}

func (d Device) String() string {
	return fmt.Sprintf("%s (%s) [%s] - Support: [%s, %s]", d.Modelname, strings.Join(d.Codenames, "; "), d.Cpu, d.MinOS.String(), d.MaxOS.String())
}
func ParseSystemOnChips() ([]Cpu, error) {
	rawcpus, error := htmltable.NewSliceFromURL[TableCPU](IOSSystemOnChipsPage)
	if error != nil {
		log.Fatal("[ParseSystemOnChips] %s", error.Error())
		return nil, error
	}
	var out []Cpu
	processorTitleRegex, err := regexp2.Compile("^(A[0-9]+X?(?: Fusion| Bionic| Pro)?)", regexp2.None)
	if err != nil {
		log.Fatalf("regexp compile failed: %s", err.Error())
	}

	for _, cpu := range rawcpus {
		match, _ := processorTitleRegex.FindStringMatch(cpu.Label)
		if match != nil {
			label := match.GroupByNumber(1).Capture.String()
			code := strings.Replace(label, " ", "_", -1)
			out = append(out, Cpu{Label: label, Code: code})
		}
	}
	return out, nil
}
func ParseiOSVersionHistory(client *http.Client) []version.OSVersion {
	log.Debugf("[ParseiOSVersionHistory] Fetching data (GET) from %s", IOSVersionHistoryURL)
	res, err := client.Get(IOSVersionHistoryURL)
	if err != nil {
		log.Fatal("[ParseiOSVersionHistory] %s", err.Error())
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("[ParseiOSVersionHistory] status code error: %d %s", res.StatusCode, res.Status)
	}
	var versions []version.OSVersion
	verregex := regexp.MustCompile(`^[0-9]+\.[0-9]+(?:\.[0-9]+)?$`)
	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	} else {
		doc.Find(".wikitable th[id]").Each(func(cellidx int, cell *goquery.Selection) {
			idattr, exists := cell.Attr("id")
			if exists {
				trimmed := strings.TrimSpace(idattr)
				if verregex.MatchString(trimmed) {
					version, err := version.OSVersionFromString(trimmed)
					if err == nil {
						versions = append(versions, version)
					} else {
						log.Errorf("[ParseiOSVersionHistory] Error parsing string version %s", trimmed)
					}
				} else {
					log.Errorf("[ParseiOSVersionHistory] Cell 'id' attr does not match a version string: %s", trimmed)
				}
			} else { // should never happen, as the selector query must filter cells that have 'id'
				log.Error("[ParseiOSVersionHistory] Cell matched but does not have 'id' attribute")
			}
		})
	}
	return versions
}
func ParseSingleIOSVersionPage(page string, client *http.Client) []version.OSVersion {
	// take all .wikitable that have row(0).th(0).textContent == Version
	// then take all first td,th/textContent, matching regex \d+.\d+.\d+
	// trim any <sup>.*</sup footnotes
	res, err := client.Get(page)
	if err != nil {
		log.Fatal("[ParseSingleIOSVersionPage] %s", err.Error())
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("[ParseSingleIOSVersionPage] status code error: %d %s", res.StatusCode, res.Status)
	}
	var versions []version.OSVersion
	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	} else {
		verregex := regexp.MustCompile("[0-9]+.[0-9]+(?:.[0-9]+)?")
		supregex := regexp.MustCompile("<sup>.*</sup>")
		brregex := regexp.MustCompile("<br/?>")
		doc.Find(".wikitable").Each(func(tableidx int, table *goquery.Selection) {
			firstHeaderCellContent := table.Find("tr").First().Find("th").First().Text()
			if strings.TrimSpace(firstHeaderCellContent) == "Version" { // this is the right table
				table.Find("tr").Each(func(rowidx int, row *goquery.Selection) {
					if rowidx == 0 {
						return
					}
					// some cell has two or more version values, separated by <br>
					firstCellInnerHtml, _ := row.Find("td,th").First().Html()
					firstCellInnerHtmlNoSup := supregex.ReplaceAllString(firstCellInnerHtml, "")
					firstCellVersions := brregex.Split(firstCellInnerHtmlNoSup, -1)
					for _, rawversion := range firstCellVersions {
						version_match := verregex.FindString(rawversion)
						if len(version_match) > 0 {
							verobj, _ := version.OSVersionFromString(version_match)
							versions = append(versions, verobj)
						}
					}
				})
			}
		})
	}
	log.Infof("[ParseSingleIOSVersionPage] page %s: found %d versions", page, len(versions))
	return versions
}
func ParseiOSVersionHistory2(client *http.Client) []version.OSVersion {
	var versions []version.OSVersion
	for _, page := range IOSVersionPages {
		versions = append(versions, ParseSingleIOSVersionPage(page, client)...)
	}
	return versions
}
func ParseListOfIphoneModelsTable(client *http.Client) []Device {
	res, err := client.Get(ListOfIphoneModelsURL)
	if err != nil {
		log.Fatal("[ParseListOfIphoneModelsTable] %s", err.Error())
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("[ParseListOfIphoneModelsTable] status code error: %d %s", res.StatusCode, res.Status)
	}
	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	} else {
		var gDevices []Device
		doc.Find(".wikitable").Each(func(tableidx int, table *goquery.Selection) {
			var devices []Device
			log.Tracef("[ParseListOfIphoneModelsTable] Parsing .wikitable %d", tableidx)
			rows := table.Find("tr")
			var modelsRow *goquery.Selection
			firstRowFirstCellText := strings.TrimSpace(rows.First().Find("th").First().Text())
			if firstRowFirstCellText == "Model" {
				modelsRow = rows.First()
				modelsRow.Find("th").Each(func(colidx int, col *goquery.Selection) {
					if colidx == 0 {
						return
					}
					log.Tracef("Appending model %s", col.Text())
					devices = append(devices, Device{Modelname: strings.TrimSpace(col.Text())})
				})
				for rowidx := 1; rowidx < rows.Length(); rowidx++ {
					row := rows.Eq(rowidx)
					firstcoltext := strings.TrimSpace(row.Find("th").Eq(0).Text())
					secondcoltext := strings.TrimSpace(row.Find("th").Eq(1).Text())
					if firstcoltext == "Processor" && secondcoltext == "Chip" {
						modelidx := 0
						row.Find("td").Each(func(tdidx int, td *goquery.Selection) {
							cellRawContent := td.Text()
							removeNotesRegex := regexp.MustCompile(`(?m)\[\d+\]*`)
							content := removeNotesRegex.ReplaceAllString(cellRawContent, "")
							colspan := 1
							colspanstr, exists := td.Attr("colspan")
							cpu := strings.TrimSpace(content)
							if exists {
								colspan, _ = strconv.Atoi(colspanstr)
							}
							for i := 0; i < colspan; i++ {
								devices[modelidx].Cpu = cpu
								modelidx++
							}
						})
						if modelidx != len(devices) {
							log.Errorf("Found CPU cells count (%d) doesn't match devices count (%d). Last device added: %s", modelidx, len(devices), devices[modelidx-1].String())
						}
					} else if firstcoltext == "Hardware strings" {
						modelidx := 0
						row.Find("td").Each(func(tdidx int, td *goquery.Selection) {
							content, _ := td.Html()
							carriageRegex, err := regexp2.Compile("iPhone[0-9]+,[0-9]+", regexp2.None)
							if err != nil {
								log.Fatalf("[ParseListOfIphoneModelsTable] '%s' column: regex compile error: %s", firstcoltext, err.Error())
							}
							match, err := carriageRegex.FindStringMatch(content)
							if err != nil {
								log.Fatalf("[ParseListOfIphoneModelsTable] '%s' column: regex match error: %s", firstcoltext, err.Error())
							}
							if match == nil {
								log.Fatalf("[ParseListOfIphoneModelsTable] '%s' column: regex no match: %s", firstcoltext, content)
							}
							devices[modelidx].Codenames = append(devices[modelidx].Codenames, strings.TrimSpace(match.String()))
							match2, err := carriageRegex.FindNextMatch(match)
							if match2 != nil {
								log.Infof("[ParseListOfIphoneModelsTable] Found multiple codenames for model %s: %s and %s", devices[modelidx].Modelname, match.String(), match2.String())
								devices[modelidx].Codenames = append(devices[modelidx].Codenames, strings.TrimSpace(match2.String()))
							} else if err != nil {
								log.Warnf("")
							}
							modelidx++
						})
					} else if firstcoltext == "Initial release operating system" {
						modelidx := 0
						row.Find("td").Each(func(tdidx int, td *goquery.Selection) {
							verregex := regexp.MustCompile("[0-9]+.[0-9]+(?:.[0-9]+)?")
							content := td.Text()
							verstr := verregex.FindString(content)
							if err != nil {
								log.Fatalf("[ParseListOfIphoneModelsTable] '%s' regex match error: %s", firstcoltext, err.Error())
							}
							if len(verstr) == 0 {
								log.Fatalf("[ParseListOfIphoneModelsTable] '%s' regex no match: %s", firstcoltext, content)
							}
							minos, err := version.OSVersionFromString(verstr)
							if err != nil {
								log.Warnf("[ParseListOfIphoneModelsTable] '%s' Error parsing min OS version from string: %s", firstcoltext, err.Error())
								return
							}
							colspan := 1
							colspanstr, exists := td.Attr("colspan")
							if exists {
								colspan, _ = strconv.Atoi(colspanstr)
							}
							for i := 0; i < colspan; i++ {
								devices[modelidx].MinOS = minos
								modelidx++
							}
						})
					} else if firstcoltext == "Latest release operating system" {
						modelidx := 0
						row.Find("td").Each(func(tdidx int, td *goquery.Selection) {
							verregex := regexp.MustCompile("[0-9]+.[0-9]+(?:.[0-9]+)?")
							content := td.Text()
							verstr := verregex.FindString(content)
							if err != nil {
								log.Fatalf("[ParseListOfIphoneModelsTable] '%s' regex match error: %s", firstcoltext, err.Error())
							}
							if len(verstr) == 0 {
								log.Fatalf("[ParseListOfIphoneModelsTable] '%s' regex no match: %s", firstcoltext, content)
							}
							maxos, err := version.OSVersionFromString(verstr)
							if err != nil {
								log.Warnf("[ParseListOfIphoneModelsTable] '%s' Error parsing min OS version from string: %s", firstcoltext, err.Error())
								return
							}
							colspan := 1
							colspanstr, exists := td.Attr("colspan")
							if exists {
								colspan, _ = strconv.Atoi(colspanstr)
							}
							for i := 0; i < colspan; i++ {
								devices[modelidx].MaxOS = maxos
								modelidx++
							}
						})
					}
				}
				gDevices = append(gDevices, devices...)
			} else {
				log.Debugf("[ParseListOfIphoneModelsTable] First row in table %d does not start with 'Models' cell: %s", tableidx, firstRowFirstCellText)
				return
			}
		})
		for _, dev := range gDevices {
			log.Debugf("[ParseListOfIphoneModelsTable] Device: ", dev.String())
		}
		return gDevices
	}
	return nil
}
