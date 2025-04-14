package wikipedia

import (
	"appledata/packages/version"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/dlclark/regexp2"
	htmltable "github.com/nfx/go-htmltable"
	log "github.com/sirupsen/logrus"
)

const DEFAULT_WIKISTR = "https://en.wikipedia.org/wiki"

func WikiBase() string {
	wikistr, exists := os.LookupEnv("WIKI_BASE")
	if !exists {
		wikistr = DEFAULT_WIKISTR
	}
	return wikistr
}
func WikiPageURL(urlstr string) string {
	trailingSlash := regexp.MustCompile(`/$`)
	leadingSlash := regexp.MustCompile(`^/`)
	base := trailingSlash.ReplaceAllString(WikiBase(), "")
	path := leadingSlash.ReplaceAllString(urlstr, "")

	return fmt.Sprintf("%s/%s", base, path)
}

var WikiBaseUrl = WikiBase()

var IOSVersionPages []string = []string{
	"IPhone_OS_2",
	"IPhone_OS_3",
	"IPhone_OS_4",
	"IPhone_OS_5",
	"IPhone_OS_6",
	"IPhone_OS_7",
	"IPhone_OS_8",
	"IPhone_OS_9",
	"IPhone_OS_10",
	"IPhone_OS_11",
	"IPhone_OS_12",
	"IPhone_OS_13",
	"IPhone_OS_14",
	"IPhone_OS_15",
	"IPhone_OS_16",
	"IOS_17",
	"IOS_18",
}

type TableCPU struct {
	Label            string `header:"System-on-chip"`
	Ram              string `header:"RAM"`
	RamType          string `header:"RAM type"`
	StorageType      string `header:"Storage type"`
	ModelName        string `header:"Model"`
	LatestIosVersion string `header:"Highest supported iOS"`
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
func ParseSystemOnChips(client *http.Client) ([]Cpu, error) {
	var IOSSystemOnChipsPage string = WikiPageURL("List_of_iPhone_models#iPhone_systems-on-chips")
	res, err := client.Get(IOSSystemOnChipsPage)
	if err != nil {
		log.Fatalf("[ParseSystemOnChips] %s", err.Error())
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("[ParseSystemOnChips] %s status code error: %d %s", res.Request.URL.String(), res.StatusCode, res.Status)
	}

	rawcpus, error := htmltable.NewSliceFromResponse[TableCPU](res)
	if error != nil {
		log.Fatalf("[ParseSystemOnChips][NewSliceFromResponse] %s", error.Error())
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
	var IOSVersionHistoryURL string = WikiPageURL("wiki/IOS_version_history")
	log.Debugf("[ParseiOSVersionHistory] Fetching data (GET) from %s", IOSVersionHistoryURL)
	res, err := client.Get(IOSVersionHistoryURL)
	if err != nil {
		log.Fatalf("[ParseiOSVersionHistory] %s", err.Error())
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
	for _, pagepath := range IOSVersionPages {
		page := WikiPageURL(pagepath)
		versions = append(versions, ParseSingleIOSVersionPage(page, client)...)
	}
	return versions
}
func ParseListOfIphoneModelsTable(client *http.Client) []Device {
	var ListOfIphoneModelsURL string = WikiPageURL("/List_of_iPhone_models")
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
			log.Debugf("[ParseListOfIphoneModelsTable] Parsing .wikitable %d", tableidx)
			rows := table.Find("tr")
			var modelsRow *goquery.Selection
			firstRowFirstCellText := strings.TrimSpace(rows.First().Find("th").First().Text())
			firstRowSecondCellText := strings.TrimSpace(rows.First().Find("th").Eq(1).Text())
			if firstRowFirstCellText == "Model" && strings.HasPrefix(firstRowSecondCellText, "iPhone") {
				modelsRow = rows.First()
				modelsRow.Find("th").Each(func(colidx int, col *goquery.Selection) {
					if colidx == 0 {
						return
					}
					log.Debugf("Appending model %s", col.Text())
					devices = append(devices, Device{Modelname: strings.TrimSpace(col.Text())})
				})
				for rowidx := 1; rowidx < rows.Length(); rowidx++ {
					row := rows.Eq(rowidx)
					firstcoltext := strings.TrimSpace(row.Find("th").Eq(0).Text())
					secondcoltext := strings.TrimSpace(row.Find("th").Eq(1).Text())
					thirdcoltext := strings.TrimSpace(row.Find("th").Eq(2).Text())
					if strings.EqualFold(firstcoltext, "Performance") && strings.EqualFold(secondcoltext, "Chip") && strings.EqualFold(thirdcoltext, "Chip Name") {
						log.Debugf("[ParseListOfIphoneModelsTable] tabel[%d] Parsing CPU info raw", tableidx)
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
					} else if strings.EqualFold(firstcoltext, "Basic Info") && strings.EqualFold(secondcoltext, "Hardware strings") {
						log.Debugf("[ParseListOfIphoneModelsTable] table[%d] Parsing hardware strings row", tableidx)
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
					} else if strings.EqualFold(firstcoltext, "operating system") && strings.EqualFold(secondcoltext, "initial") {
						log.Debugf("[ParseListOfIphoneModelsTable] table[%d] Parsing iOS release range row", tableidx)
						modelidx := 0
						initialRow := row
						latestRow := row.Next()
						rangeRows := []*goquery.Selection{initialRow, latestRow}
						for ridx := 0; ridx < len(rangeRows); ridx++ {
							theRow := rangeRows[ridx]
							theRow.Find("td").Each(func(tdidx int, td *goquery.Selection) {
								verregex := regexp.MustCompile("[0-9]+.[0-9]+(?:.[0-9]+)?")
								content := td.Text()
								verstr := verregex.FindString(content)
								if err != nil {
									log.Fatalf("[ParseListOfIphoneModelsTable] '%s' regex match error: %s", firstcoltext, err.Error())
								}
								if len(verstr) == 0 {
									log.Fatalf("[ParseListOfIphoneModelsTable] '%s' regex no match: %s", firstcoltext, content)
								}
								oslimit, err := version.OSVersionFromString(verstr)
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
									if ridx == 0 {
										devices[modelidx].MinOS = oslimit
									}else {
										devices[modelidx].MaxOS = oslimit
									}
									modelidx++
								}
							})
							modelidx = 0
						}
					} // else unhandled row
				} // end of rows loop
				gDevices = append(gDevices, devices...)
			} else {
				log.Debugf("[ParseListOfIphoneModelsTable] First row in table %d does not start with 'Model | iPhone*' cells: %s", tableidx, firstRowFirstCellText)
				return
			}
		})
		for _, dev := range gDevices {
			log.Debugf("[ParseListOfIphoneModelsTable] Device: %s", dev.String())
		}
		return gDevices
	}
	return nil
}
