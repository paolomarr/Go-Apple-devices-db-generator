package theiphonewiki

import (
	"fmt"
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"github.com/dlclark/regexp2"
	log "github.com/sirupsen/logrus"
)

var BaseURL string = "https://theiphonewiki.com"
var Processor_page string = "/wiki/Application_Processor"

type Cpu struct {
	Code  string
	Label string
}

func ParseProcessorsPages(client *http.Client) ([]Cpu, error) {
	// Request the HTML page.
	pageurl := fmt.Sprintf("%s/%s", BaseURL, Processor_page)
	log.Debugf("[ParseProcessorsPages] Fetching data (GET) from %s", pageurl)
	res, err := client.Get(pageurl)
	if err != nil {
		// log.Fatal(err)
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	var retcpus []Cpu
	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		// log.Fatal(err)
		return nil, err
	}
	doc.Find("h5 span.mw-headline").Each(func(cpuidx int, s *goquery.Selection) {
		title := s.Text()
		processorTitleRegex, err := regexp2.Compile("^([^ ]+) (Apple A[0-9]+X?(?: Fusion| Bionic)?)", regexp2.None)
		if err != nil {
			log.Fatalf("regexp compile failed: %s", err.Error())
		}
		match, _ := processorTitleRegex.FindStringMatch(title)
		if match != nil {
			code := match.GroupByNumber(1).Capture.String()
			label := match.GroupByNumber(2).Capture.String()
			retcpus = append(retcpus, Cpu{Label: label, Code: code})
		}
	})
	return retcpus, nil
}
