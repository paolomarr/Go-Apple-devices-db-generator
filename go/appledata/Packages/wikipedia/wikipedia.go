package wikipedia

import (
	"appledata/Packages/version"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"time"

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

func httpGetWithRetry(client *http.Client, url string, maxRetries int, defaultWait time.Duration) (*http.Response, error) {
    for attempt := 0; attempt <= maxRetries; attempt++ {
        res, err := client.Get(url)
        if err != nil {
            return nil, err
        }
        if res.StatusCode != 429 {
            return res, nil
        }
        // Handle 429: Too Many Requests
        wait := defaultWait
        if retryAfter := res.Header.Get("Retry-After"); retryAfter != "" {
            if secs, err := strconv.Atoi(retryAfter); err == nil {
                wait = time.Duration(secs) * time.Second
            } else {
				log.Warnf("Failed to parse header value 'Retry-After: %s' to an integer value. Using default wait period.", retryAfter)
			}
        } else {
			log.Warnf("No 'Retry-After' header in response, using default wait period.")
		}
        log.Warnf("Received 429 for %s, waiting %v before retrying (attempt %d/%d)", url, wait, attempt+1, maxRetries)
        res.Body.Close()
        time.Sleep(wait)
    }
    return nil, fmt.Errorf("max retries exceeded for %s", url)
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
	"IOS_26",
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
	ModelNumbers []string
	MinOS     version.OSVersion
	MaxOS     version.OSVersion
}
type Cpu struct {
	Code  string
	Label string
}

func (d Device) String() string {
	return fmt.Sprintf("%s (%s) codeNames[%s] modelNumbers[%s] osRange[%s, %s]", d.Modelname, strings.Join(d.Codenames, "; "), strings.Join(d.ModelNumbers, "; "), d.Cpu, d.MinOS.String(), d.MaxOS.String())
}
func ParseSystemOnChips(client *http.Client) ([]Cpu, error) {
	var IOSSystemOnChipsPage string = WikiPageURL("List_of_iPhone_models#iPhone_systems-on-chips")
	res, err := httpGetWithRetry(client, IOSSystemOnChipsPage, 3, 60* time.Second)
	// res, err := client.Get(IOSSystemOnChipsPage)
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
	res, err := httpGetWithRetry(client, IOSVersionHistoryURL, 3, 60 * time.Second)
	// res, err := client.Get(IOSVersionHistoryURL)
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
func ParseSingleIOSVersionPage(page string, client *http.Client) []version.IOSVersion {
	// take all .wikitable that have row(0).th(0).textContent == Version
	// then take all first td,th/textContent, matching regex \d+.\d+.\d+
	// trim any <sup>.*</sup footnotes
	res, err := httpGetWithRetry(client, page, 3, 60 * time.Second)
	// res, err := client.Get(page)
	if err != nil {
		log.Fatalf("[ParseSingleIOSVersionPage] page[%s] HTTP GET error: %s", page, err.Error())
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("[ParseSingleIOSVersionPage] page[%s] HTTP status code error: %d %s", page, res.StatusCode, res.Status)
	}
	var versions []version.IOSVersion
	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	} else {
		verregex := regexp.MustCompile(`[0-9]+\.[0-9]+(?:\.[0-9]+)?`)
		supregex := regexp.MustCompile("<sup>.*</sup>")
		brregex := regexp.MustCompile("<br/?>")
		doc.Find(".wikitable").Each(func(tableidx int, table *goquery.Selection) {
			iosVersion := version.IOSVersion{}
			firstHeaderCellContent := table.Find("tr").First().Find("th").First().Text()
			if strings.TrimSpace(firstHeaderCellContent) == "Version" { // this is the right table
				buildNumberRowsLeft := 1
				table.Find("tr").Each(func(rowidx int, row *goquery.Selection) {
					if rowidx == 0 {
						return
					}
					firstcell := row.Find("th, td").First()
					// Some table have inner header rows. They can be spotted from the colspan > 1 property
					if firstcell.AttrOr("colspan", "1") != "1" {
						// simply discard the row
						return
					}
					firstCellInnerHtml, _ := firstcell.Html()
					versionStringMatched := false
					// on latest iOS tables (e.g. 18/26) the version header cell has a data-sort-value property. Let's try and get that, as 
					// in those tables the header cell content may be more complex than the simple "X.Y.Z" string.
					dataSortValueAttr, exists := row.Find("th").First().Attr("data-sort-value")
					if exists {
						version_match := verregex.FindString(dataSortValueAttr)
						if len(version_match) > 0 {
							verobj, err := version.OSVersionFromString(version_match)
							if err != nil {
								log.Errorf("[ParseSingleIOSVersionPage] page[%s] Error parsing version from 'data-sort-value' attr string %s", page, dataSortValueAttr)
							}else {
								iosVersion.Version = verobj
								versionStringMatched = true
								// versions = append(versions, verobj)
							}
						}
					} else {
						firstCellInnerHtmlNoSup := supregex.ReplaceAllString(firstCellInnerHtml, "")
						// some cell has two or more version values, separated by <br>
						firstCellVersions := brregex.Split(firstCellInnerHtmlNoSup, -1)
						for _, rawversion := range firstCellVersions {
							version_match := verregex.FindString(rawversion)
							if len(version_match) > 0 {
								verobj, err := version.OSVersionFromString(version_match)
								if err != nil {
									log.Errorf("[ParseSingleIOSVersionPage] page[%s] Error parsing version from cell content string %s", page, rawversion)
								}else {
									iosVersion.Version = verobj
									versionStringMatched = true
									// versions = append(versions, verobj)
								}
							}
							// else {
							// 	log.Errorf("[ParseSingleIOSVersionPage] page[%s] No version match for raw string %s", page, rawversion)
							// }
						}	
					}
					// try and fetch build numbers, too
					rowspanAttr, exists := firstcell.Attr("rowspan")
					buildNumberCellContent := ""
					var buildNumberCell *goquery.Selection
					if versionStringMatched {
						buildNumberCell = firstcell.Next()
						buildNumberCellContent, _ = buildNumberCell.Html()
						nosup := supregex.ReplaceAllString(buildNumberCellContent, "")
						buildNumbers := brregex.Split(nosup, -1)
						for _, buildNumber := range buildNumbers {
							bnobj, err := version.BuildNumberFromString(buildNumber)
							if err != nil {
								log.Errorf("[ParseSingleIOSVersionPage] page[%s] Error parsing build number from cell content string %s (first row)", page, buildNumber)
							}else {
								iosVersion.Builds = append(iosVersion.Builds, bnobj)
							}
						}
						if exists { // multiple build number rows for this version
							buildNumberRowsLeft, _ = strconv.Atoi(rowspanAttr)
						} else {
							buildCardinalityString := "one-build"
							if len(iosVersion.Builds) > 1 {
								buildCardinalityString = "multi-build"
							}
							log.Debugf("[ParseSingleIOSVersionPage] page[%s] Appending %s version %s", page, buildCardinalityString, iosVersion.String())
							versions = append(versions, iosVersion)
							iosVersion = version.IOSVersion{}
							return
						}
					}
					buildNumberCellContent, _ = firstcell.Html()
					nosup := supregex.ReplaceAllString(buildNumberCellContent, "")
					buildNumbers := brregex.Split(nosup, -1)
					for _, buildNumber := range buildNumbers {
						bnobj, err := version.BuildNumberFromString(buildNumber)
						if err != nil {
							log.Errorf("[ParseSingleIOSVersionPage] page[%s] Error parsing build number from cell content string %s (next-to-first row)", page, buildNumber)
						}else {
							iosVersion.Builds = append(iosVersion.Builds, bnobj)
						}
					}
					buildNumberRowsLeft--
					if buildNumberRowsLeft == 0 {
						log.Debugf("[ParseSingleIOSVersionPage] page[%s] Appending multi-build version %s", page, iosVersion.String())
						versions = append(versions, iosVersion)
						iosVersion = version.IOSVersion{}
						return
					}
				})
			}
		})
	}
	log.Infof("[ParseSingleIOSVersionPage] page[%s] versions[%d]", page, len(versions))
	return versions
}
func ParseiOSVersionHistory2(client *http.Client) []version.IOSVersion {
	var versions []version.IOSVersion
	for _, pagepath := range IOSVersionPages {
		page := WikiPageURL(pagepath)
		versions = append(versions, ParseSingleIOSVersionPage(page, client)...)
	}
	return versions
}
func parseHardwareStrings(hardwareStringsRow *goquery.Selection, devices *[]Device) {
	headerCellText := "Hardware strings"
	hardwareStringsRow.Find("td").Each(func(cellidx int, tcell *goquery.Selection) {
		content, _ := tcell.Html()
		carriageRegex, err := regexp2.Compile("iPhone[0-9]+,[0-9]+", regexp2.None)
		if err != nil {
			log.Fatalf("[ParseListOfIphoneModelsTable] '%s' column: regex compile error: %s", headerCellText, err.Error())
		}
		match, err := carriageRegex.FindStringMatch(content)
		if err != nil {
			log.Fatalf("[ParseListOfIphoneModelsTable] '%s' column: regex match error: %s", headerCellText, err.Error())
		}
		if match == nil {
			log.Fatalf("[ParseListOfIphoneModelsTable] '%s' column: regex no match: %s", headerCellText, content)
		}	
		for match != nil {
			(*devices)[cellidx].Codenames = append((*devices)[cellidx].Codenames, strings.TrimSpace(match.String()))
			match, _ = carriageRegex.FindNextMatch(match)
		}
		if (len((*devices)[cellidx].Codenames) > 1) {
			log.Infof("[ParseListOfIphoneModelsTable] model[%s] multiple codenames[%s]", (*devices)[cellidx].Modelname, strings.Join((*devices)[cellidx].Codenames, ", "))
		}
	})
}
func parseModelNumbers(modelNumbersRow *goquery.Selection, devices *[]Device) {
	headerCellText := "Model numbers"
	modelNumbersRow.Find("td").Each(func(cellidx int, tcell *goquery.Selection) {
		content, _ := tcell.Html()
		carriageRegex, err := regexp2.Compile("A[0-9]+", regexp2.None)
		if err != nil {
			log.Fatalf("[parseModelNumbers] '%s' column: regex compile error: %s", headerCellText, err.Error())
		}
		mnFound := 0
		match, _ := carriageRegex.FindStringMatch(content)
		for match != nil {
			(*devices)[cellidx].ModelNumbers = append((*devices)[cellidx].ModelNumbers, strings.TrimSpace(match.String()))
			mnFound++
			match, _ = carriageRegex.FindNextMatch(match)
		}
		log.Debugf("[parseModelNumbers] Found %d model numbers for model %s", mnFound, (*devices)[cellidx].Modelname)
	})
}
func parseOSVersionRange(initialLatestRows *goquery.Selection, devices *[]Device) {
	headerCellText := "Operating System"
	for ridx := 0; ridx < 2; ridx++ {
		deviceIdx := 0
		theRow := initialLatestRows.Eq(ridx)
		theRow.Find("td").Each(func(tdidx int, td *goquery.Selection) {
			verregex, err := regexp2.Compile(`(?:iOS|iPhoneOS) (?<version>[0-9]+\.[0-9]+(?:\.[0-9]+)?)`, regexp2.None)
			if err != nil {
				log.Fatalf("[parseOSVersionRange] '%s' Error compiling regex: %s", headerCellText, err.Error())
			}
			content := td.Text()
			match, _ := verregex.FindStringMatch(content)
			for match != nil {
				oslimit, err := version.OSVersionFromString(match.GroupByName("version").Capture.String())
				if err != nil {
					log.Warnf("[parseOSVersionRange] '%s' Error parsing min OS version from string: %s", headerCellText, err.Error())
					return
				}
				colspan, _ := strconv.Atoi(td.AttrOr("colspan", "1"))
				for i := 0; i < colspan; i++ {
					if ridx == 0 { // initial
						(*devices)[deviceIdx].MinOS = oslimit
					}else { // latest
						(*devices)[deviceIdx].MaxOS = oslimit
					}
					deviceIdx++
				}
				match, _ = verregex.FindNextMatch(match)
			}
		})
	}
}
func parseBasicInfoSlice(basicInfoRows *goquery.Selection, devices *[]Device) {
	basicInfoRows.Each(func(rowidx int, row *goquery.Selection) {
		row.Find("th").Each(func(cellidx int, hcell *goquery.Selection) {
			headerCellText := strings.TrimSpace(hcell.Text())
			if strings.EqualFold(headerCellText, "Hardware strings") {
				// from rowspan attribute/property of the header cell, take a slice of basicInfoRows and feed it to the next function
				parseHardwareStrings(row, devices)
			} else if(strings.EqualFold(headerCellText, "Model number")) {
				parseModelNumbers(row, devices)
			} else if(strings.EqualFold(headerCellText, "Initial")) {
				// OS version range consists on two rows, "Initial" and "Latest"
				parseOSVersionRange(basicInfoRows.Slice(rowidx, rowidx+2), devices)
			}
		})
	})
}
func ParseListOfIphoneModelsTable(client *http.Client) []Device {
	var ListOfIphoneModelsURL string = WikiPageURL("/List_of_iPhone_models")
	res, err := client.Get(ListOfIphoneModelsURL)
	if err != nil {
		log.Fatalf("[ParseListOfIphoneModelsTable] %s", err.Error())
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
					headerCellsSelection := row.Find("th")
					headerCellsSelection.Each(func(cellidx int, cell *goquery.Selection) {
						headerCellText := strings.TrimSpace(cell.Text())
						if strings.EqualFold(headerCellText, "Basic Info") {
							rowspan, _ := strconv.Atoi(cell.AttrOr("rowspan", "0"))
							parseBasicInfoSlice(rows.Slice(rowidx, rowidx + rowspan), &devices)
						} else if strings.EqualFold(headerCellText, "Chip Name") {
							log.Debugf("[ParseListOfIphoneModelsTable] tabel[%d] Parsing CPU info row", tableidx)
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
						} 
					})
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
