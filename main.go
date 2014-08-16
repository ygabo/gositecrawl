package main

import (
	"code.google.com/p/go.net/html"
	"encoding/json"
	"fmt"
	"net/http"
	// "regexp"
	"bytes"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"sync"
	// "time"
)

var (
	MAIN_LINK      = "https://www.tastytrade.com"
	TASTY_BITE_URL = "https://www.tastytrade.com/tt/shows/tasty-bites/episodes?_=1408128889680&locale=en-CA"

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

	waitGroup sync.WaitGroup

	tastyShow TastyBiteShow
)

type TastyBiteShow struct {
	Episodes []Episode
	Pages    string // paginated
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

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func (ep Episode) String() string {
	val, _ := json.Marshal(ep)
	return string(val)
}

func grabTastyBiteEpisodeLinks(page int) {
	//pages := 6
	lastPage := ""
	tastyurl := TASTY_BITE_URL
	if page > 0 {
		tastyurl = TASTY_BITE_URL + "&page=" + strconv.Itoa(page)
	}

	response, err := http.Get(tastyurl)
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
					tastyShow.Episodes = append(tastyShow.Episodes, episode)
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
				tastyShow.Pages = lastPage
			}
		}
	}

}

func saveToFile(name string, data []byte) {
	// open output file
	fo, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	// close fo on exit and check for its returned error
	defer func() {
		if err := fo.Close(); err != nil {
			panic(err)
		}
	}()

	// make a buffer to keep chunks that are read
	start := 0
	end := len(data)
	for {
		// write a chunk
		n, _ := fo.Write(data[start:end])
		start = start + n
		if start >= end {
			break
		}
	}
}

func (episode *Episode) grabEpisode() {

	useDummy := false
	var data nopCloser
	var err error
	var all []byte
	defer waitGroup.Done()

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

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	fmt.Println("Grabbing Tasty Bite Links...")
	grabTastyBiteEpisodeLinks(0)

	if tastyShow.Pages != "" {
		pages, _ := strconv.Atoi(tastyShow.Pages)
		for i := 1; i <= pages; i++ {
			grabTastyBiteEpisodeLinks(i)
		}
	}

	fmt.Println("Done. --------")

	fmt.Println("Querying everything.")
	for i := 0; i < len(tastyShow.Episodes); i++ {
		waitGroup.Add(1)
		go tastyShow.Episodes[i].grabEpisode()
	}

	waitGroup.Wait()
	b, _ := json.MarshalIndent(tastyShow, "", "  ")
	//fmt.Println(string(b))

	saveToFile("tasty-bite.txt", []byte(b))
}
