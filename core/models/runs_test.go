package models_test

import (
	"context"
	"os"
	"testing"

	"github.com/buger/jsonparser"
	"github.com/gomodule/redigo/redis"
	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/goflow/test"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionCreationAndUpdating(t *testing.T) {
	ctx, rt, db, _ := testsuite.Get()

	defer testsuite.Reset(testsuite.ResetData)

	assetsJSON, err := os.ReadFile("testdata/session_test_flows.json")
	require.NoError(t, err)

	flowJSON, _, _, err := jsonparser.Get(assetsJSON, "flows", "[0]")
	require.NoError(t, err)
	flow := testdata.InsertFlow(db, testdata.Org1, flowJSON)

	oa, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshFlows)
	require.NoError(t, err)

	flowSession, sprint1 := test.NewSessionBuilder().WithAssets(assetsJSON).WithFlow("c49daa28-cf70-407a-a767-a4c1360f4b01").
		WithContact(testdata.Bob.UUID, flows.ContactID(testdata.Bob.ID), "Bob", "eng", "").MustBuild()

	tx := db.MustBegin()

	hookCalls := 0
	hook := func(context.Context, *sqlx.Tx, *redis.Pool, *models.OrgAssets, []*models.Session) error {
		hookCalls++
		return nil
	}

	modelSessions, err := models.WriteSessions(ctx, rt, tx, oa, []flows.Session{flowSession}, []flows.Sprint{sprint1}, hook)
	require.NoError(t, err)
	assert.Equal(t, 1, hookCalls)

	require.NoError(t, tx.Commit())

	session := modelSessions[0]

	assert.Equal(t, models.FlowTypeMessaging, session.SessionType())
	assert.Equal(t, testdata.Bob.ID, session.ContactID())
	assert.Equal(t, models.SessionStatusWaiting, session.Status())
	assert.Equal(t, flow.ID, session.CurrentFlowID())
	assert.NotNil(t, session.CreatedOn())
	assert.Nil(t, session.EndedOn())
	assert.False(t, session.Responded())
	assert.NotNil(t, session.WaitStartedOn())
	assert.NotNil(t, session.WaitExpiresOn())
	assert.NotNil(t, session.Timeout())

	// check that matches what is in the db
	testsuite.AssertQuery(t, db, `SELECT status, session_type, current_flow_id, responded, ended_on FROM flows_flowsession`).
		Columns(map[string]interface{}{"status": "W", "session_type": "M", "current_flow_id": int64(flow.ID), "responded": false, "ended_on": nil})

	flowSession, err = session.FlowSession(rt.Config, oa.SessionAssets(), oa.Env())
	require.NoError(t, err)

	flowSession, sprint2, err := test.ResumeSession(flowSession, assetsJSON, "no")
	require.NoError(t, err)

	tx = db.MustBegin()

	err = session.Update(ctx, rt, tx, oa, flowSession, sprint2, hook)
	require.NoError(t, err)
	assert.Equal(t, 2, hookCalls)

	require.NoError(t, tx.Commit())

	assert.Equal(t, models.SessionStatusWaiting, session.Status())
	assert.Equal(t, flow.ID, session.CurrentFlowID())
	assert.True(t, session.Responded())
	assert.NotNil(t, session.WaitStartedOn())
	assert.NotNil(t, session.WaitExpiresOn())
	assert.Nil(t, session.Timeout()) // this wait doesn't have a timeout

	flowSession, err = session.FlowSession(rt.Config, oa.SessionAssets(), oa.Env())
	require.NoError(t, err)

	flowSession, sprint3, err := test.ResumeSession(flowSession, assetsJSON, "yes")
	require.NoError(t, err)

	tx = db.MustBegin()

	err = session.Update(ctx, rt, tx, oa, flowSession, sprint3, hook)
	require.NoError(t, err)
	assert.Equal(t, 3, hookCalls)

	require.NoError(t, tx.Commit())

	assert.Equal(t, models.SessionStatusCompleted, session.Status())
	assert.Equal(t, models.NilFlowID, session.CurrentFlowID()) // no longer "in" a flow
	assert.True(t, session.Responded())
	assert.NotNil(t, session.CreatedOn())
	assert.Nil(t, session.WaitStartedOn())
	assert.Nil(t, session.WaitExpiresOn())
	assert.Nil(t, session.Timeout())
	assert.NotNil(t, session.EndedOn())

	// check that matches what is in the db
	testsuite.AssertQuery(t, db, `SELECT status, session_type, current_flow_id, responded FROM flows_flowsession`).
		Columns(map[string]interface{}{"status": "C", "session_type": "M", "current_flow_id": nil, "responded": true})
}

func TestSingleSprintSession(t *testing.T) {
	ctx, rt, db, _ := testsuite.Get()

	defer testsuite.Reset(testsuite.ResetData)

	assetsJSON, err := os.ReadFile("testdata/session_test_flows.json")
	require.NoError(t, err)

	flowJSON, _, _, err := jsonparser.Get(assetsJSON, "flows", "[1]")
	require.NoError(t, err)
	testdata.InsertFlow(db, testdata.Org1, flowJSON)

	oa, err := models.GetOrgAssetsWithRefresh(ctx, rt, testdata.Org1.ID, models.RefreshFlows)
	require.NoError(t, err)

	flowSession, sprint1 := test.NewSessionBuilder().WithAssets(assetsJSON).WithFlow("8b1b02a0-e217-4d59-8ecb-3b20bec69cf4").
		WithContact(testdata.Bob.UUID, flows.ContactID(testdata.Bob.ID), "Bob", "eng", "").MustBuild()

	tx := db.MustBegin()

	hookCalls := 0
	hook := func(context.Context, *sqlx.Tx, *redis.Pool, *models.OrgAssets, []*models.Session) error {
		hookCalls++
		return nil
	}

	modelSessions, err := models.WriteSessions(ctx, rt, tx, oa, []flows.Session{flowSession}, []flows.Sprint{sprint1}, hook)
	require.NoError(t, err)
	assert.Equal(t, 1, hookCalls)

	require.NoError(t, tx.Commit())

	session := modelSessions[0]

	assert.Equal(t, models.FlowTypeMessaging, session.SessionType())
	assert.Equal(t, testdata.Bob.ID, session.ContactID())
	assert.Equal(t, models.SessionStatusCompleted, session.Status())
	assert.Equal(t, models.NilFlowID, session.CurrentFlowID())
	assert.NotNil(t, session.CreatedOn())
	assert.NotNil(t, session.EndedOn())
	assert.False(t, session.Responded())
	assert.Nil(t, session.WaitStartedOn())
	assert.Nil(t, session.WaitExpiresOn())
	assert.Nil(t, session.Timeout())

	// check that matches what is in the db
	testsuite.AssertQuery(t, db, `SELECT status, session_type, current_flow_id, responded FROM flows_flowsession`).
		Columns(map[string]interface{}{"status": "C", "session_type": "M", "current_flow_id": nil, "responded": false})
}