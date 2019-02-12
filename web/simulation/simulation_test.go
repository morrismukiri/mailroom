package simulation

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nyaruka/mailroom/config"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/web"
	"github.com/stretchr/testify/assert"
)

const (
	startBody = `
	{
		"org_id": 1,
		"trigger": {
			"contact": {
				"created_on": "2000-01-01T00:00:00.000000000-00:00",
				"fields": {},
				"id": 1234567,
				"language": "eng",
				"name": "Ben Haggerty",
				"timezone": "America/Guayaquil",
				"urns": [
					"tel:+12065551212"
				],
				"uuid": "ba96bf7f-bc2a-4873-a7c7-254d1927c4e3"
			},
			"environment": {
				"allowed_languages": [
					"eng",
					"fra"
				],
				"date_format": "YYYY-MM-DD",
				"default_language": "eng",
				"time_format": "hh:mm",
				"timezone": "America/Los_Angeles"
			},
			"flow": {
				"name": "Favorites",
				"uuid": "9de3663f-c5c5-4c92-9f45-ecbc09abcc85"
			},
			"triggered_on": "2000-01-01T00:00:00.000000000-00:00",
			"type": "manual"
		}
	}`

	resumeBody = `
	{
		"org_id": 1,
		"resume": {
			"contact": {
                "created_on": "2000-01-01T00:00:00.000000000-00:00",
                "fields": {},
                "id": 1234567,
                "language": "eng",
                "name": "Ben Haggerty",
                "timezone": "America/Guayaquil",
                "urns": [
                    "tel:+12065551212"
                ],
                "uuid": "ba96bf7f-bc2a-4873-a7c7-254d1927c4e3"
            },
            "environment": {
                "allowed_languages": [
                    "eng",
                    "fra"
                ],
                "date_format": "YYYY-MM-DD",
                "default_language": "eng",
                "time_format": "hh:mm",
                "timezone": "America/New_York"
            },
            "msg": {
                "channel": {
                    "name": "Twitter",
                    "uuid": "0f661e8b-ea9d-4bd3-9953-d368340acf91"
                },
                "text": "I like blue!",
                "urn": "tel:+12065551212",
                "uuid": "9bf91c2b-ce58-4cef-aacc-281e03f69ab5"
            },
            "resumed_on": "2000-01-01T00:00:00.000000000-00:00",
            "type": "msg"
		},
		"assets": {
			"channels": [
				{
					"uuid": "440099cf-200c-4d45-a8e7-4a564f4a0e8b",
					"name": "Test Channel",
					"address": "+18005551212",
					"schemes": ["tel"],
					"roles": ["send", "receive", "call"],
					"country": "US"
				}
			]
		},
		"session": $$SESSION$$
	}`

	minLegacyDef = `
	{
		"flow": {
			"base_language": "eng",
			"action_sets": [{
				"y": 0,
				"x": 100,
				"destination": null,
				"uuid": "e41e7aad-de93-4cc0-ae56-d6af15ba1ac5",
				"actions": [{
					"uuid": "0aaa6871-15fb-408c-9f33-2d7d8f6d5baf",
					"msg": {
						"eng": "Hello world"
					},
					"type": "reply"
				}],
				"exit_uuid": "40c6cb36-bb44-479a-8ed1-d3f8df3a134d"
			}],
			"version": 8,
			"flow_type": "F",
			"entry": "e41e7aad-de93-4cc0-ae56-d6af15ba1ac5",
			"rule_sets": [],
			"metadata": {
				"uuid": "42362831-f376-4df1-b6d9-a80b102821d9",
				"expires": 10080,
				"revision": 1,
				"id": 41049,
				"name": "No ruleset flow",
				"saved_on": "2015-11-20T11:02:19.790131Z"
			}
		},
		"include_ui": true
	}
	`

	customStartBody = `
	{
		"org_id": 1,
		"trigger": {
			"contact": {
				"created_on": "2000-01-01T00:00:00.000000000-00:00",
				"fields": {},
				"id": 1234567,
				"language": "eng",
				"name": "Ben Haggerty",
				"timezone": "America/Guayaquil",
				"urns": [
					"tel:+12065551212"
				],
				"uuid": "ba96bf7f-bc2a-4873-a7c7-254d1927c4e3"
			},
			"environment": {
				"allowed_languages": [
					"eng",
					"fra"
				],
				"date_format": "YYYY-MM-DD",
				"default_language": "eng",
				"time_format": "hh:mm",
				"timezone": "America/Los_Angeles"
			},
			"msg": {
				"uuid": "2d611e17-fb22-457f-b802-b8f7ec5cda5b",
				"channel": {"uuid": "440099cf-200c-4d45-a8e7-4a564f4a0e8b", "name": "Test Channel"},
				"urn": "tel:+12065551212",
				"text": "hi there"
			},
			"flow": {
				"name": "Favorites",
				"uuid": "9de3663f-c5c5-4c92-9f45-ecbc09abcc85"
			},
			"triggered_on": "2000-01-01T00:00:00.000000000-00:00",
			"type": "msg"
		},
		"flows": [
			{
				"uuid": "9de3663f-c5c5-4c92-9f45-ecbc09abcc85",
				"legacy_definition": {
					"base_language": "eng",
					"action_sets": [{
						"y": 0,
						"x": 100,
						"destination": null,
						"uuid": "87edb79e-f46c-4970-bcd3-c715dfededb7",
						"actions": [{
							"uuid": "0aaa6871-15fb-408c-9f33-2d7d8f6d5baf",
							"msg": {
								"eng": "Your channel is @channel.name"
							},
							"type": "reply"
						}],
						"exit_uuid": "40c6cb36-bb44-479a-8ed1-d3f8df3a134d"
					}],
					"version": 8,
					"flow_type": "F",
					"entry": "87edb79e-f46c-4970-bcd3-c715dfededb7",
					"rule_sets": [],
					"metadata": {
						"uuid": "9de3663f-c5c5-4c92-9f45-ecbc09abcc85",
						"expires": 10080,
						"revision": 1,
						"id": 41049,
						"name": "No ruleset flow",
						"saved_on": "2015-11-20T11:02:19.790131Z"
					}
				}
			}
		],
		"assets": {
			"channels": [
				{
					"uuid": "440099cf-200c-4d45-a8e7-4a564f4a0e8b",
					"name": "Test Channel",
					"address": "+12065551441",
					"schemes": ["tel"],
					"roles": ["send", "receive", "call"],
					"country": "US"
				}
			]
		}
	}`
)

func TestServer(t *testing.T) {
	testsuite.Reset()
	ctx := testsuite.CTX()
	db := testsuite.DB()
	rp := testsuite.RP()
	wg := &sync.WaitGroup{}

	server := web.NewServer(ctx, config.Mailroom, db, rp, nil, wg)
	server.Start()

	// give our server time to start
	time.Sleep(time.Second)

	defer server.Stop()
	startSession := ""

	tcs := []struct {
		URL      string
		Method   string
		Body     string
		Status   int
		Response string
	}{
		{"/mr/sim/start", "GET", "", 405, "illegal"},
		{"/mr/sim/start", "POST", startBody, 200, "What is your favorite color?"},
		{"/mr/sim/resume", "GET", "", 405, "illegal"},
		{"/mr/sim/resume", "POST", resumeBody, 200, "Good choice, I like Blue too! What is your favorite beer?"},

		{"/mr/sim/start", "POST", customStartBody, 200, "Your channel is Test Channel"},

		{"/mr/flow/migrate", "GET", "", 405, "illegal"},
		{"/mr/flow/migrate", "POST", minLegacyDef, 200, `"type": "send_msg"`},
	}

	for i, tc := range tcs {
		var body io.Reader

		// in the case of a resume, we have to sub in our session body from our start
		if tc.Body == resumeBody {
			tc.Body = strings.Replace(tc.Body, "$$SESSION$$", startSession, -1)
		}

		if tc.Body != "" {
			body = bytes.NewReader([]byte(tc.Body))
		}

		req, err := http.NewRequest(tc.Method, "http://localhost:8090"+tc.URL, body)
		assert.NoError(t, err, "%d: error creating request", i)

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err, "%d: error making request", i)

		assert.Equal(t, tc.Status, resp.StatusCode, "%d: unexpected status", i)

		content, err := ioutil.ReadAll(resp.Body)
		assert.NoError(t, err, "%d: error reading body", i)

		// if this was a start and the start was successful
		if tc.Body == startBody && resp.StatusCode == 200 {
			// save the start session for use in our resume
			parsed := make(map[string]interface{})
			json.Unmarshal(content, &parsed)
			sessionJSON, _ := json.Marshal(parsed["session"])
			startSession = string(sessionJSON)
		}

		assert.True(t, strings.Contains(string(content), tc.Response), "%d: did not find string: %s in body: %s", i, tc.Response, string(content))
	}
}