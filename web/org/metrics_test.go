package org_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/nyaruka/mailroom/config"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/nyaruka/mailroom/web"

	"github.com/stretchr/testify/assert"
)

func TestMetrics(t *testing.T) {
	ctx, db, rp := testsuite.Reset()

	promToken := "2d26a50841ff48237238bbdd021150f6a33a4196"
	db.MustExec(`INSERT INTO api_apitoken(is_active, org_id, created, key, role_id, user_id) VALUES(TRUE, $1, NOW(), $2, 12, 1);`, testdata.Org1.ID, promToken)

	adminToken := "5c26a50841ff48237238bbdd021150f6a33a4199"
	db.MustExec(`INSERT INTO api_apitoken(is_active, org_id, created, key, role_id, user_id) VALUES(TRUE, $1, NOW(), $2, 8, 1);`, testdata.Org1.ID, adminToken)

	wg := &sync.WaitGroup{}
	server := web.NewServer(ctx, config.Mailroom, db, rp, nil, nil, wg)
	server.Start()

	// wait for the server to start
	time.Sleep(time.Second)
	defer server.Stop()

	tcs := []struct {
		Label    string
		URL      string
		Username string
		Password string
		Response string
		Contains []string
	}{
		{
			Label:    "no username",
			URL:      fmt.Sprintf("http://localhost:8090/mr/org/%s/metrics", testdata.Org1.UUID),
			Username: "",
			Password: "",
			Response: `{"error": "invalid authentication"}`,
		},
		{
			Label:    "invalid password",
			URL:      fmt.Sprintf("http://localhost:8090/mr/org/%s/metrics", testdata.Org1.UUID),
			Username: "metrics",
			Password: "invalid",
			Response: `{"error": "invalid authentication"}`,
		},
		{
			Label:    "invalid username",
			URL:      fmt.Sprintf("http://localhost:8090/mr/org/%s/metrics", testdata.Org1.UUID),
			Username: "invalid",
			Password: promToken,
			Response: `{"error": "invalid authentication"}`,
		},
		{
			Label:    "valid login, wrong org",
			URL:      fmt.Sprintf("http://localhost:8090/mr/org/%s/metrics", testdata.Org2.UUID),
			Username: "metrics",
			Password: promToken,
			Response: `{"error": "invalid authentication"}`,
		},
		{
			Label:    "valid login, invalid user",
			URL:      fmt.Sprintf("http://localhost:8090/mr/org/%s/metrics", testdata.Org1.UUID),
			Username: "metrics",
			Password: adminToken,
			Response: `{"error": "invalid authentication"}`,
		},
		{
			Label:    "valid",
			URL:      fmt.Sprintf("http://localhost:8090/mr/org/%s/metrics", testdata.Org1.UUID),
			Username: "metrics",
			Password: promToken,
			Contains: []string{
				`rapidpro_group_contact_count{group_name="Active",group_uuid="14f6ea01-456b-4417-b0b8-35e942f549f1",group_type="system",org="UNICEF"} 124`,
				`rapidpro_group_contact_count{group_name="Doctors",group_uuid="c153e265-f7c9-4539-9dbc-9b358714b638",group_type="user",org="UNICEF"} 121`,
				`rapidpro_channel_msg_count{channel_name="Vonage",channel_uuid="19012bfd-3ce3-4cae-9bb9-76cf92c73d49",channel_type="NX",msg_direction="out",msg_type="message",org="UNICEF"} 1`,
			},
		},
	}

	for _, tc := range tcs {
		req, _ := http.NewRequest(http.MethodGet, tc.URL, nil)
		req.SetBasicAuth(tc.Username, tc.Password)
		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err, "%s: received error", tc.Label)

		body, _ := ioutil.ReadAll(resp.Body)

		if tc.Response != "" {
			assert.Equal(t, tc.Response, string(body), "%s: response mismatch", tc.Label)
		}
		for _, contains := range tc.Contains {
			assert.Contains(t, string(body), contains, "%s: does not contain: %s", tc.Label, contains)
		}
	}
}
