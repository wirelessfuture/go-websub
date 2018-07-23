package subscriber

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	httpmock "gopkg.in/jarcoal/httpmock.v1"
)

/*

	Subscription request:

											/->[callback uri]
		Subscriber client --> mock http server [posing as hub]

	Validation, verification:
		mock --> subscriber client

*/

func TestClient_handleAckedSubscription(t *testing.T) {
	httpmock.Activate()

	/* First super basic test */
	sc := NewClient("4000")

	httpmock.RegisterResponder("POST", "http://example.com/feed",
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(202, "")
			return resp, nil
		})

	t.Run("Successful subscription", func(t *testing.T) {
		sc.topicsToSelf["http://example.com/feed"] = "http://example.com/feed"
		err := sc.SubscribeToTopic("http://example.com/feed")
		if err != nil {
			t.Error("Failed to subscribe", err)
		}
	})

	httpmock.DeactivateAndReset()
	httpmock.Activate()

	/* Second test -- tests callback works as advertised */

	var callback string

	httpmock.RegisterResponder("POST", "http://example.com/feed",
		func(req *http.Request) (*http.Response, error) {
			resp := httpmock.NewStringResponse(202, "")
			if reqBody, err := ioutil.ReadAll(req.Body); err == nil {
				values, err := url.ParseQuery(string(reqBody))
				if err != nil {
					panic(err)
				}
				callback = values.Get("hub.callback")
			}
			return resp, nil
		})

	t.Run("Everything works", func(t *testing.T) {
		sc.topicsToSelf["http://example.com/feed"] = "http://example.com/feed"
		sc.SubscribeToTopic("http://example.com/feed")

		if _, ok := sc.pendingSubs["http://example.com/feed"]; !ok {
			t.Fatal("Subscription not registered as pending")
		}

		if len(callback) == 0 {
			t.Fatal("Callback unset")
		}

		// At this point, the callback URI is up and waiting
		data := make(url.Values)
		data.Set("hub.mode", "subscribe")
		data.Set("hub.topic", "http://example.com/feed")
		data.Set("hub.challenge", "kitties")
		data.Set("hub.lease_seconds", "20")

		req, err := http.NewRequest("POST", "http://localhost:4000/callback/"+callback, strings.NewReader(data.Encode()))
		if err != nil {
			panic(err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Content-Length", strconv.Itoa(len(data.Encode())))

		httpmock.Deactivate()

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			panic(err)
		}

		if resp.StatusCode != 200 {
			t.Fatalf("Status code is %d instead of 200", resp.StatusCode)
		}

		if respBody, err := ioutil.ReadAll(resp.Body); err == nil {
			if string(respBody) != "kitties" {
				t.Fatalf("Response is {%v} instead of {kitties}", respBody)
			}
		} else {
			t.Fatalf("Failed to parse body with err {%v}", err)
		}
	})

	httpmock.DeactivateAndReset()

}