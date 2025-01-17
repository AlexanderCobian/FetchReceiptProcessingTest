package main

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

/*
ProcessReponse and GetResponse are used to unmarshal the simple single-value
JSON objects returned in response to the tested GET and POST requests
*/
type ProcessResponse struct {
	Id string `json:"id"`
}

type GetResponse struct {
	Points int64 `json:"points"`
}

// Does the heavy lifting for all of the successful-path tests. Sends the
// payload in via a POST request, reads the ID from the response, and sends
// that back in with a GET request, checking that the number of points returned
// matches the expected amount.
func testPostAndGetHelper(t *testing.T, payload []byte, expected int64) {

	// Prepare and send the POST request
	req := httptest.NewRequest(http.MethodPost, "/receipts/process", bytes.NewBuffer(payload))
	w := httptest.NewRecorder()

	processReceipt(w, req)

	// Unpack the response to the POST request into a ProcessResponse
	resp := w.Result()
	data, _ := io.ReadAll(resp.Body)

	var pr ProcessResponse
	err := json.Unmarshal(data, &pr)
	if err != nil {
		// Note error if we don't get a valid response
		t.Errorf("Invalid JSON on POST: %s", err)
	}

	// Prepare and send the GET request
	getPath := "/receipts/" + pr.Id + "/points"
	req = httptest.NewRequest(http.MethodGet, getPath, nil)
	w = httptest.NewRecorder()

	getPoints(w, req)

	// Unpack the response to the GET request into a GetResponse
	resp = w.Result()
	data, _ = io.ReadAll(resp.Body)

	var gr GetResponse
	err = json.Unmarshal(data, &gr)
	if err != nil {
		// Note error if we don't get a valid response
		t.Errorf("Invalid JSON on GET: %s", err)
	}

	// Note error if the points returned don't match our expectations.
	if gr.Points != expected {
		t.Errorf("Expected %v but got %v", expected, gr.Points)
	}

}

// Facilitates the actual POST request part of all of the badly-formatted input
// failure tests
func testBadPostHelper(t *testing.T, payload []byte) {

	// Prepare and send the POST request
	req := httptest.NewRequest(http.MethodPost, "/receipts/process", bytes.NewBuffer(payload))
	w := httptest.NewRecorder()

	processReceipt(w, req)

	// Since our expectation in this case is to receive a 400 BadRequest, note
	// error if we *don't* receive one or if the error message is wrong
	resp := w.Result()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("BadRequest header expected but not returned")
	}
	if string(data) != "The receipt is invalid." {
		t.Errorf("Error message missing or incorrect.")
	}

}

// SUCCESSFUL PATH TESTS START HERE

func TestMorningReceipt(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "08:13",
		"total": "2.65",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)
	var expected int64 = 9 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		0 + // 25 if cents divide by 25
		5 + // 5 per every two items
		1 + // Based on costs of items with descriptions %3==0
		0 + // 6 if day is odd
		0 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestSimpleReceipt(t *testing.T) {

	payload := []byte(`{
    	"retailer": "Target",
    	"purchaseDate": "2022-01-02",
    	"purchaseTime": "13:13",
    	"total": "1.25",
    	"items": [
      		{"shortDescription": "Pepsi - 12-oz", "price": "1.25"}
    	]
	}`)
	var expected int64 = 6 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		25 + // 25 if cents divide by 25
		0 + // 5 per every two items
		0 + // Based on costs of items with descriptions %3==0
		0 + // 6 if day is odd
		0 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestInlineExample1(t *testing.T) {
	payload := []byte(`{
		"retailer": "Target",
		"purchaseDate": "2022-01-01",
		"purchaseTime": "13:01",
		"items": [
			{"shortDescription": "Mountain Dew 12PK","price": "6.49"},
			{"shortDescription": "Emils Cheese Pizza","price": "12.25"},
			{"shortDescription": "Knorr Creamy Chicken","price": "1.26"},
			{"shortDescription": "Doritos Nacho Cheese","price": "3.35"},
			{"shortDescription": "   Klarbrunn 12-PK 12 FL OZ  ","price": "12.00"}
		],
		"total": "35.35"
	}`)
	var expected int64 = 6 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		0 + // 25 if cents divide by 25
		(2 * 5) + // 5 per every two items
		(3 + 3) + // Based on costs of items with descriptions %3==0
		6 + // 6 if day is odd
		0 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)
}

func TestInlineExample2(t *testing.T) {
	payload := []byte(`{
		"retailer": "M&M Corner Market",
		"purchaseDate": "2022-03-20",
		"purchaseTime": "14:33",
		"items": [
			{"shortDescription": "Gatorade","price": "2.25"},
			{"shortDescription": "Gatorade","price": "2.25"},
			{"shortDescription": "Gatorade","price": "2.25"},
			{"shortDescription": "Gatorade","price": "2.25"}
		],
		"total": "9.00"
	}`)
	var expected int64 = 14 + // 1 per alphanumeric char in retailer
		50 + // 50 if cents == 00
		25 + // 25 if cents divide by 25
		(2 * 5) + // 5 per every two items
		0 + // Based on costs of items with descriptions %3==0
		0 + // 6 if day is odd
		10 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)
}

func TestRetailerNameOnly(t *testing.T) {

	payload := []byte(`{
    	"retailer": "abcdefghijklmnopqrstuvwxyz",
    	"purchaseDate": "2025-01-02",
    	"purchaseTime": "00:00",
    	"total": "0.01",
    	"items": [
      		{"shortDescription": "item", "price": "0.01"}
    	]
	}`)
	var expected int64 = 26 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		0 + // 25 if cents divide by 25
		0 + // 5 per every two items
		0 + // Based on costs of items with descriptions %3==0
		0 + // 6 if day is odd
		0 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestOnTheDollar(t *testing.T) {

	payload := []byte(`{
    	"retailer": "a",
    	"purchaseDate": "2025-01-02",
    	"purchaseTime": "00:00",
    	"total": "1.00",
    	"items": [
      		{"shortDescription": "item", "price": "1.00"}
    	]
	}`)
	var expected int64 = 1 + // 1 per alphanumeric char in retailer
		50 + // 50 if cents == 00
		25 + // 25 if cents divide by 25
		0 + // 5 per every two items
		0 + // Based on costs of items with descriptions %3==0
		0 + // 6 if day is odd
		0 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestOnTheQuarter(t *testing.T) {

	payload := []byte(`{
    	"retailer": "a",
    	"purchaseDate": "2025-01-02",
    	"purchaseTime": "00:00",
    	"total": "1.75",
    	"items": [
      		{"shortDescription": "item", "price": "1.75"}
    	]
	}`)
	var expected int64 = 1 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		25 + // 25 if cents divide by 25
		0 + // 5 per every two items
		0 + // Based on costs of items with descriptions %3==0
		0 + // 6 if day is odd
		0 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestItemCount(t *testing.T) {

	payload := []byte(`{
    	"retailer": "a",
    	"purchaseDate": "2025-01-02",
    	"purchaseTime": "00:00",
    	"total": "0.11",
    	"items": [
      		{"shortDescription": "item", "price": "0.01"},
			{"shortDescription": "item", "price": "0.01"},
			{"shortDescription": "item", "price": "0.01"},
			{"shortDescription": "item", "price": "0.01"},
			{"shortDescription": "item", "price": "0.01"},
			{"shortDescription": "item", "price": "0.01"},
			{"shortDescription": "item", "price": "0.01"},
			{"shortDescription": "item", "price": "0.01"},
			{"shortDescription": "item", "price": "0.01"},
			{"shortDescription": "item", "price": "0.01"},
			{"shortDescription": "item", "price": "0.01"}
    	]
	}`)
	var expected int64 = 1 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		0 + // 25 if cents divide by 25
		(5 * 5) + // 5 per every two items
		0 + // Based on costs of items with descriptions %3==0
		0 + // 6 if day is odd
		0 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestPerItemPoints(t *testing.T) {

	payload := []byte(`{
    	"retailer": "a",
    	"purchaseDate": "2025-01-02",
    	"purchaseTime": "00:00",
    	"total": "2290.28",
    	"items": [
      		{"shortDescription": "abc", "price": "5.26"},
			{"shortDescription": "abc", "price": "82.74"},
			{"shortDescription": "abc", "price": "924.25"},
			{"shortDescription": "abcd", "price": "1278.03"}
    	]
	}`)
	var expected int64 = 1 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		0 + // 25 if cents divide by 25
		(5 * 2) + // 5 per every two items

		// Based on costs of items with descriptions %3==0
		int64(math.Ceil(5.26*0.2)) +
		int64(math.Ceil(82.74*0.2)) +
		int64(math.Ceil(924.25*0.2)) +

		0 + // 6 if day is odd
		0 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestOddDate(t *testing.T) {

	payload := []byte(`{
    	"retailer": "a",
    	"purchaseDate": "2025-01-03",
    	"purchaseTime": "00:00",
    	"total": "0.01",
    	"items": [
      		{"shortDescription": "item", "price": "0.01"}
    	]
	}`)
	var expected int64 = 1 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		0 + // 25 if cents divide by 25
		0 + // 5 per every two items
		0 + // Based on costs of items with descriptions %3==0
		6 + // 6 if day is odd
		0 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestAfternoonBonus1(t *testing.T) {

	payload := []byte(`{
    	"retailer": "a",
    	"purchaseDate": "2025-01-02",
    	"purchaseTime": "14:00",
    	"total": "0.01",
    	"items": [
      		{"shortDescription": "item", "price": "0.01"}
    	]
	}`)
	var expected int64 = 1 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		0 + // 25 if cents divide by 25
		0 + // 5 per every two items
		0 + // Based on costs of items with descriptions %3==0
		0 + // 6 if day is odd
		0 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestAfternoonBonus2(t *testing.T) {

	payload := []byte(`{
    	"retailer": "a",
    	"purchaseDate": "2025-01-02",
    	"purchaseTime": "14:01",
    	"total": "0.01",
    	"items": [
      		{"shortDescription": "item", "price": "0.01"}
    	]
	}`)
	var expected int64 = 1 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		0 + // 25 if cents divide by 25
		0 + // 5 per every two items
		0 + // Based on costs of items with descriptions %3==0
		0 + // 6 if day is odd
		10 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestAfternoonBonus3(t *testing.T) {

	payload := []byte(`{
    	"retailer": "a",
    	"purchaseDate": "2025-01-02",
    	"purchaseTime": "15:59",
    	"total": "0.01",
    	"items": [
      		{"shortDescription": "item", "price": "0.01"}
    	]
	}`)
	var expected int64 = 1 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		0 + // 25 if cents divide by 25
		0 + // 5 per every two items
		0 + // Based on costs of items with descriptions %3==0
		0 + // 6 if day is odd
		10 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestAfternoonBonus4(t *testing.T) {

	payload := []byte(`{
    	"retailer": "a",
    	"purchaseDate": "2025-01-02",
    	"purchaseTime": "16:00",
    	"total": "0.01",
    	"items": [
      		{"shortDescription": "item", "price": "0.01"}
    	]
	}`)
	var expected int64 = 1 + // 1 per alphanumeric char in retailer
		0 + // 50 if cents == 00
		0 + // 25 if cents divide by 25
		0 + // 5 per every two items
		0 + // Based on costs of items with descriptions %3==0
		0 + // 6 if day is odd
		0 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

func TestOneWithEverything(t *testing.T) {

	payload := []byte(`{
    	"retailer": "Fetch",
    	"purchaseDate": "2025-01-03",
    	"purchaseTime": "15:00",
    	"total": "10.00",
    	"items": [
      		{"shortDescription": "Thing1", "price": "4.00"},
			{"shortDescription": "Thing2", "price": "6.00"}
    	]
	}`)
	var expected int64 = 5 + // 1 per alphanumeric char in retailer
		50 + // 50 if cents == 00
		25 + // 25 if cents divide by 25
		5 + // 5 per every two items
		1 + 2 + // Based on costs of items with descriptions %3==0
		6 + // 6 if day is odd
		10 // 10 if purchase was made between 2 and 4 PM

	testPostAndGetHelper(t, payload, expected)

}

// FAILURE PATH TESTS START HERE

func TestMissingElement(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "08:13",
		"total": "2.65",
	}`)

	testBadPostHelper(t, payload)

}

func TestExtraElement(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens<",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "08:13",
		"total": "2.65",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		],
		"foo": 13.37
	}`)

	testBadPostHelper(t, payload)

}

func TestBadRetailer1(t *testing.T) {

	payload := []byte(`{
		"retailer": "",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "08:13",
		"total": "2.65",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestBadRetailer2(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens<",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "08:13",
		"total": "2.65",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestBadDate1(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022-02-29",
		"purchaseTime": "08:13",
		"total": "2.65",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestBadDate2(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022/01/02",
		"purchaseTime": "08:13",
		"total": "2.65",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestBadTime1(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "0813",
		"total": "2.65",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestBadTime2(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "25:13",
		"total": "2.65",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestBadTotal1(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "0813",
		"total": "265",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestBadTotal2(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "0813",
		"total": "2.657",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestBadItemDesc1(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "0813",
		"total": "2.65",
		"items": [
			{"shortDescription": "", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestBadItemDesc2(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "0813",
		"total": "2.65",
		"items": [
			{"shortDescription": "Pepsi < 12-oz", "price": "1.25"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestBadItemPrice1(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "0813",
		"total": "2.65",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "125"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestBadItemPrice2(t *testing.T) {

	payload := []byte(`{
		"retailer": "Walgreens",
		"purchaseDate": "2022-01-02",
		"purchaseTime": "0813",
		"total": "2.65",
		"items": [
			{"shortDescription": "Pepsi - 12-oz", "price": "1.257"},
			{"shortDescription": "Dasani", "price": "1.40"}
		]
	}`)

	testBadPostHelper(t, payload)

}

func TestMissingGet(t *testing.T) {
	getPath := "/receipts/fake-id/points"
	req := httptest.NewRequest(http.MethodGet, getPath, nil)
	w := httptest.NewRecorder()

	getPoints(w, req)

	resp := w.Result()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("NotFound header expected but not returned")
	}
	if string(data) != "No receipt found for that ID." {
		t.Errorf("Error message missing or incorrect.")
	}
}
