package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"code.google.com/p/go.net/html"
	// "regexp"
	"bytes"
	"io/ioutil"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"time"
)

var (
	MAIN_LINK = "https://www.tastytrade.com"

	ATTR_CLASS       = "class"
	ATTR_DATA_RETURN = "data-return"
	ATTR_NAME        = "name"
	ATTR_VALUE       = "value"
	ATTR_CONTENT     = "content"
	ATTR_PROPERTY    = "property"

	NODE_LI    = "li"
	NODE_DIV   = "div"
	NODE_PARAM = "param"
	NODE_META  = "meta"
	NODE_H3    = "h3"

	TYPE_EPISODE          = "episode_thumbnail"
	TYPE_FLASH_VARS       = "flashVars"
	TYPE_TITLE            = "og:title"
	TYPE_FULL_TITLE       = "title"
	TYPE_IMAGE            = "og:image"
	TYPE_DESCRIPTION      = "description"
	TYPE_SHOW_DESCRIPTION = "og:description"
	TYPE_LAST             = "last"

	KEY_MEDIA_ID = "mediaId"
	KEY_PAGE_NUM = "page"

	shows         []TastyTradeShow
	showSyncGroup sync.WaitGroup
)

type TastyTradeShow struct {
	Title    string    `json:"title"`
	FileName string    `json:"file_name"`
	Link     string    `json:"link"`
	Pages    string    `json:"pages"`
	Episodes []Episode `json:"episodes"`
}
type Episode struct {
	Title     string `json:"title"`
	FullTitle string `json:"full_title"`
	MediaId   string `json:"media_id"`
	Image     string `json:"image"`
	Link      string `json:"link"`
	FlashVar  string `json:"flashvar"`
	Date      string `json:"date"`
}

func (ep Episode) String() string {
	val, _ := json.Marshal(ep)
	return string(val)
}

func grabEpisodeLinks(show *TastyTradeShow, showURL string, page int) {
	//pages := 6
	lastPage := show.Pages
	fullUrl := showURL
	if page > 0 {
		fullUrl = showURL + "&page=" + strconv.Itoa(page)
	}

	response, err := http.Get(fullUrl)
	if err == nil {
		defer response.Body.Close()
		z := html.NewTokenizer(response.Body)
		for {
			if z.Next() == html.ErrorToken {
				break // done
			}
			token := z.Token()
			if token.Data == NODE_DIV {
				classIndex := 0
				if len(token.Attr) > 1 && token.Attr[classIndex].Key == ATTR_CLASS && token.Attr[classIndex].Val == TYPE_EPISODE {
					episode := Episode{Link: MAIN_LINK + token.Attr[1].Val}

					// Get the corresponding Title and Date for this episode -- Pretty hacky :p
					for z.Next() != html.ErrorToken && z.Token().Data != NODE_H3 {
					}
					z.Next()
					episode.Title = z.Token().String()
					for i := 0; i < 4; i++ {
						z.Next()
					}
					episode.Date = z.Token().String()
					show.Episodes = append(show.Episodes, episode)
				}
			} else if token.Data == NODE_LI && len(token.Attr) > 0 && token.Attr[0].Key == ATTR_CLASS && token.Attr[0].Val == TYPE_LAST && lastPage == "" {
				// Grab how many pages this show has.
				// <li class='last'>
				// 	<a href="/tt/shows/good-trade-bad-trade/episodes?locale=en-US&amp;page=2">Last &raquo;</a>
				// </li>
				z.Next()
				z.Next()
				m, _ := url.ParseQuery(z.Token().Attr[0].Val)
				lastPage = m[KEY_PAGE_NUM][0]
				show.Pages = lastPage
			}
		}
	}

}

func (episode *Episode) grabEpisode(group *sync.WaitGroup) {

	useDummy := false
	var data nopCloser
	var err error
	var all []byte
	defer group.Done()

	if useDummy {
		all, err = ioutil.ReadFile("dummy.html")
		data = nopCloser{bytes.NewReader(all)}
	} else {
		response, err2 := http.Get(episode.Link)
		data = nopCloser{response.Body}
		err = err2
	}

	if err == nil {
		defer data.Close()
		z := html.NewTokenizer(data)
		for {
			if z.Next() == html.ErrorToken {
				break // done
			}
			token := z.Token()

			if token.Data == NODE_PARAM {
				// Grab the flash Variables, it contains the mediaId
				//<param name="flashVars" value="playerForm=b35804e&amp;autoplay=false&amp;mediaId=021d7759ed5b4756b5886c5748da2824">
				if len(token.Attr) > 1 && token.Attr[0].Key == ATTR_NAME && token.Attr[0].Val == TYPE_FLASH_VARS {
					episode.FlashVar = token.Attr[1].Val

					// parse the flashvar as if it is a url query, then get the media ID
					m, _ := url.ParseQuery(episode.FlashVar)
					episode.MediaId = m[KEY_MEDIA_ID][0]
				}
			} else if token.Data == NODE_META {
				// Grab Title, Show Name, Image, description
				// <meta content='AMZN Iron Condor ' property='og:title'>
				propertyIndex := 1
				contentIndex := 0
				if len(token.Attr) > 1 && token.Attr[propertyIndex].Key == ATTR_PROPERTY && token.Attr[propertyIndex].Val == TYPE_TITLE {
					episode.Title = token.Attr[contentIndex].Val
				}
				if len(token.Attr) > 1 && token.Attr[propertyIndex].Key == ATTR_NAME && token.Attr[propertyIndex].Val == TYPE_FULL_TITLE {
					episode.FullTitle = token.Attr[contentIndex].Val
				}
				if len(token.Attr) > 1 && token.Attr[propertyIndex].Key == ATTR_PROPERTY && token.Attr[propertyIndex].Val == TYPE_IMAGE {
					episode.Image = token.Attr[contentIndex].Val
				}
			}
		}
	} else {
		fmt.Println("ERROR")
	}

	// fmt.Println("Done.")
	// d, _ := json.MarshalIndent(episode, "", "  ")
	// fmt.Println(string(d))
}

func fetchShow(show *TastyTradeShow, URL string) {

	grabEpisodeLinks(show, URL, 0)

	var waitGroup sync.WaitGroup

	defer showGroup.Done()
	fetchEpisodes := func() {
		for i := 0; i < len(show.Episodes); i++ {
			if show.Episodes[i].MediaId == "" {
				waitGroup.Add(1)
				go show.Episodes[i].grabEpisode(&waitGroup)
			}
		}
		waitGroup.Wait()
	}

	fmt.Println("Querying " + show.FileName + " ... pg 1")
	fetchEpisodes()

	if show.Pages != "" {
		pages, _ := strconv.Atoi(show.Pages)
		for i := 2; i <= pages; i++ {
			grabEpisodeLinks(show, URL, i)

			fmt.Println("Querying " + show.FileName + " ... pg " + strconv.Itoa(i))
			fetchEpisodes()
		}
	}

	b, _ := json.MarshalIndent(show, "", "  ")
	fmt.Println("Completed: " + show.FileName + " Total Episodes: " + strconv.Itoa(len(show.Episodes)))
	t := time.Now().Local()
	fileName := "output/" + t.Format("20060102") + "-" + show.FileName + ".txt"
	saveToFile(fileName, []byte(b))
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	dat, _ := ioutil.ReadFile("data/archives.json")
	json.Unmarshal(dat, &shows)

	for i := 0; i < len(shows); i++ {
		showSyncGroup.Add(1)
		go fetchShow(&shows[i], shows[i].Link)
	}

	showSyncGroup.Wait()
}
