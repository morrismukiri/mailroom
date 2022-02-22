package models_test

import (
	"fmt"
	"testing"

	"github.com/nyaruka/goflow/assets"
	"github.com/nyaruka/goflow/test"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/testsuite"
	"github.com/nyaruka/mailroom/testsuite/testdata"

	"github.com/olivere/elastic/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetContactIDsForQueryPage(t *testing.T) {
	ctx, rt, _, _ := testsuite.Get()

	es := testsuite.NewMockElasticServer()
	defer es.Close()

	client, err := elastic.NewClient(
		elastic.SetURL(es.URL()),
		elastic.SetHealthcheck(false),
		elastic.SetSniff(false),
	)
	require.NoError(t, err)

	oa, err := models.GetOrgAssets(ctx, rt, testdata.Org1.ID)
	require.NoError(t, err)

	tcs := []struct {
		Group             assets.GroupUUID
		ExcludeIDs        []models.ContactID
		Query             string
		Sort              string
		ExpectedESRequest string
		MockedESResponse  string
		ExpectedContacts  []models.ContactID
		ExpectedTotal     int64
		ExpectedError     string
	}{
		{
			Group: testdata.AllContactsGroup.UUID,
			Query: "george",
			ExpectedESRequest: `{
				"_source": false,
				"from": 0,
				"query": {
					"bool": {
						"must": [
							{
								"term": {
									"org_id": 1
								}
							},
							{
								"term": {
									"is_active": true
								}
							},
							{
								"term": {
									"groups": "d1ee73f0-bdb5-47ce-99dd-0c95d4ebf008"
								}
							},
							{
								"match": {
									"name": {
										"query": "george"
									}
								}
							}
						]
					}
				},
				"size": 50,
				"sort": [
					{
						"id": {
							"order": "desc"
						}
					}
				],
				"track_total_hits": true
			}`,
			MockedESResponse: fmt.Sprintf(`{
				"_scroll_id": "DXF1ZXJ5QW5kRmV0Y2gBAAAAAAAbgc0WS1hqbHlfb01SM2lLTWJRMnVOSVZDdw==",
				"took": 2,
				"timed_out": false,
				"_shards": {
				  "total": 1,
				  "successful": 1,
				  "skipped": 0,
				  "failed": 0
				},
				"hits": {
				  "total": 1,
				  "max_score": null,
				  "hits": [
					{
					  "_index": "contacts",
					  "_type": "_doc",
					  "_id": "%d",
					  "_score": null,
					  "_routing": "1",
					  "sort": [
						15124352
					  ]
					}
				  ]
				}
			}`, testdata.George.ID),
			ExpectedContacts: []models.ContactID{testdata.George.ID},
			ExpectedTotal:    1,
		},
		{
			Group:      testdata.BlockedContactsGroup.UUID,
			ExcludeIDs: []models.ContactID{testdata.Bob.ID, testdata.Cathy.ID},
			Query:      "age > 32",
			Sort:       "-age",
			ExpectedESRequest: `{
				"_source": false,
				"from": 0,
				"query": {
					"bool": {
						"must": [
							{
								"term": {
									"org_id": 1
								}
							},
							{
								"term": {
									"is_active": true
								}
							},
							{
								"term": {
									"groups": "9295ebab-5c2d-4eb1-86f9-7c15ed2f3219"
								}
							},
							{
								"nested": {
									"path": "fields",
									"query": {
										"bool": {
											"must": [
												{
													"term": {
														"fields.field": "903f51da-2717-47c7-a0d3-f2f32877013d"
													}
												},
												{
													"range": {
														"fields.number": {
															"from": 32,
															"include_lower": false,
															"include_upper": true,
															"to": null
														}
													}
												}
											]
										}
									}
								}
							}
						],
						"must_not": {
							"ids": {
								"type": "_doc",
								"values": [
									"10001",
									"10000"
								]
							}
						}
					}
				},
				"size": 50,
				"sort": [
					{
						"fields.number": {
							"nested": {
								"filter": {
									"term": {
										"fields.field": "903f51da-2717-47c7-a0d3-f2f32877013d"
									}
								},
								"path": "fields"
							},
							"order": "desc"
						}
					}
				],
				"track_total_hits": true
			}`,
			MockedESResponse: fmt.Sprintf(`{
				"_scroll_id": "DXF1ZXJ5QW5kRmV0Y2gBAAAAAAAbgc0WS1hqbHlfb01SM2lLTWJRMnVOSVZDdw==",
				"took": 2,
				"timed_out": false,
				"_shards": {
				  "total": 1,
				  "successful": 1,
				  "skipped": 0,
				  "failed": 0
				},
				"hits": {
				  "total": 1,
				  "max_score": null,
				  "hits": [
					{
					  "_index": "contacts",
					  "_type": "_doc",
					  "_id": "%d",
					  "_score": null,
					  "_routing": "1",
					  "sort": [
						15124352
					  ]
					}
				  ]
				}
			}`, testdata.George.ID),
			ExpectedContacts: []models.ContactID{testdata.George.ID},
			ExpectedTotal:    1,
		},
		{
			Query:         "goats > 2", // no such contact field
			ExpectedError: "error parsing query: goats > 2: can't resolve 'goats' to attribute, scheme or field",
		},
	}

	for i, tc := range tcs {
		es.NextResponse = tc.MockedESResponse

		_, ids, total, err := models.GetContactIDsForQueryPage(ctx, client, oa, tc.Group, tc.ExcludeIDs, tc.Query, tc.Sort, 0, 50)

		if tc.ExpectedError != "" {
			assert.EqualError(t, err, tc.ExpectedError)
		} else {
			assert.NoError(t, err, "%d: error encountered performing query", i)
			assert.Equal(t, tc.ExpectedContacts, ids, "%d: ids mismatch", i)
			assert.Equal(t, tc.ExpectedTotal, total, "%d: total mismatch", i)

			test.AssertEqualJSON(t, []byte(tc.ExpectedESRequest), []byte(es.LastRequestBody), "%d: ES request mismatch", i)
		}
	}
}

func TestGetContactIDsForQuery(t *testing.T) {
	ctx, rt, _, _ := testsuite.Get()

	es := testsuite.NewMockElasticServer()
	defer es.Close()

	client, err := elastic.NewClient(
		elastic.SetURL(es.URL()),
		elastic.SetHealthcheck(false),
		elastic.SetSniff(false),
	)
	require.NoError(t, err)

	oa, err := models.GetOrgAssets(ctx, rt, 1)
	require.NoError(t, err)

	tcs := []struct {
		query               string
		limit               int
		expectedRequestURL  string
		expectedRequestBody string
		mockedESResponse    string
		expectedContacts    []models.ContactID
		expectedError       string
	}{
		{
			query:              "george",
			limit:              -1,
			expectedRequestURL: "/_search/scroll",
			expectedRequestBody: `{
				"_source":false,
				"query": {
					"bool": {
						"must": [
							{
								"term": {
									"org_id": 1
								}
							},
							{
								"term": {
									"is_active": true
								}
							},
							{
								"term": {
									"status": "A"
								}
							},
							{
								"match": {
									"name": {
										"query": "george"
									}
								}
							}
						]
					}
				},
				"sort":["_doc"]
			}`,
			mockedESResponse: fmt.Sprintf(`{
				"_scroll_id": "DXF1ZXJ5QW5kRmV0Y2gBAAAAAAAbgc0WS1hqbHlfb01SM2lLTWJRMnVOSVZDdw==",
				"took": 2,
				"timed_out": false,
				"_shards": {
				  "total": 1,
				  "successful": 1,
				  "skipped": 0,
				  "failed": 0
				},
				"hits": {
				  "total": 1,
				  "max_score": null,
				  "hits": [
					{
					  "_index": "contacts",
					  "_type": "_doc",
					  "_id": "%d",
					  "_score": null,
					  "_routing": "1",
					  "sort": [
						15124352
					  ]
					}
				  ]
				}
			}`, testdata.George.ID),
			expectedContacts: []models.ContactID{testdata.George.ID},
		}, {
			query:              "nobody",
			limit:              -1,
			expectedRequestURL: "/contacts/_search?routing=1&scroll=15m&size=10000",
			expectedRequestBody: `{
				"_source":false,
				"query": {
					"bool": {
						"must": [
							{
								"term": {
									"org_id": 1
								}
							},
							{
								"term": {
									"is_active": true
								}
							},
							{
								"term": {
									"status": "A"
								}
							},
							{
								"match": {
									"name": {
										"query": "nobody"
									}
								}
							}
						]
					}
				},
				"sort":["_doc"]
			}`,
			mockedESResponse: `{
				"_scroll_id": "DXF1ZXJ5QW5kRmV0Y2gBAAAAAAAbgc0WS1hqbHlfb01SM2lLTWJRMnVOSVZDdw==",
				"took": 2,
				"timed_out": false,
				"_shards": {
				  "total": 1,
				  "successful": 1,
				  "skipped": 0,
				  "failed": 0
				},
				"hits": {
				  "total": 0,
				  "max_score": null,
				  "hits": []
				}
			}`,
			expectedContacts: []models.ContactID{},
		},
		{
			query:              "george",
			limit:              1,
			expectedRequestURL: "/contacts/_search?routing=1",
			expectedRequestBody: `{
				"_source": false,
				"from": 0,
				"query": {
					"bool": {
						"must": [
							{
								"term": {
									"org_id": 1
								}
							},
							{
								"term": {
									"is_active": true
								}
							},
							{
								"term": {
									"status": "A"
								}
							},
							{
								"match": {
									"name": {
										"query": "george"
									}
								}
							}
						]
					}
				},
				"size": 1
			}`,
			mockedESResponse: fmt.Sprintf(`{
				"hits": {
					"total": 1,
					"max_score": null,
					"hits": [
						{
							"_index": "contacts",
							"_type": "_doc",
							"_id": "%d",
							"_score": null,
							"_routing": "1",
							"sort": [
							15124352
							]
						}
					]
				}
			}`, testdata.George.ID),
			expectedContacts: []models.ContactID{testdata.George.ID},
		},
		{
			query:         "goats > 2", // no such contact field
			limit:         -1,
			expectedError: "error parsing query: goats > 2: can't resolve 'goats' to attribute, scheme or field",
		},
	}

	for i, tc := range tcs {
		es.NextResponse = tc.mockedESResponse

		ids, err := models.GetContactIDsForQuery(ctx, client, oa, tc.query, tc.limit)

		if tc.expectedError != "" {
			assert.EqualError(t, err, tc.expectedError)
		} else {
			assert.NoError(t, err, "%d: error encountered performing query", i)
			assert.Equal(t, tc.expectedContacts, ids, "%d: ids mismatch", i)

			assert.Equal(t, tc.expectedRequestURL, es.LastRequestURL, "%d: request URL mismatch", i)
			test.AssertEqualJSON(t, []byte(tc.expectedRequestBody), []byte(es.LastRequestBody), "%d: request body mismatch", i)
		}
	}
}
