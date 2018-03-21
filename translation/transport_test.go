package translation

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Comcast/webpa-common/wrp"

	"github.com/stretchr/testify/assert"
)

func TestRequestGetPayload(t *testing.T) {
	t.Run("EmptyNames", func(t *testing.T) {
		assert := assert.New(t)

		p, e := requestGetPayload("", "")
		assert.EqualValues(ErrEmptyNames, e)
		assert.Nil(p)
	})

	t.Run("GET", func(t *testing.T) {
		assert := assert.New(t)

		p, e := requestGetPayload("n0,n1", "")
		assert.Nil(e)

		expectedBytes, err := json.Marshal(&getWDMP{Command: CommandGet, Names: []string{"n0", "n1"}})

		if err != nil {
			panic(err)
		}

		assert.EqualValues(expectedBytes, p)
	})

	t.Run("GETAttrs", func(t *testing.T) {
		assert := assert.New(t)

		p, e := requestGetPayload("n0,n1", "attr0")
		assert.Nil(e)

		expectedBytes, err := json.Marshal(&getWDMP{Command: CommandGetAttrs, Names: []string{"n0", "n1"}, Attributes: "attr0"})

		if err != nil {
			panic(err)
		}

		assert.EqualValues(expectedBytes, p)
	})
}

func TestHandleResponse(t *testing.T) {
	assert := assert.New(t)

	//When the Status code from XMiDT is not 200, just forward the response code and body
	t.Run("StatusNotOK", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		response := &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Body:       ioutil.NopCloser(bytes.NewBufferString("t")),
			Header:     http.Header{"X-test": []string{"test"}},
		}

		err := encodeResponse(context.TODO(), recorder, response)

		assert.Nil(err)
		assert.EqualValues(http.StatusServiceUnavailable, recorder.Code)
		assert.EqualValues("t", recorder.Body.String())
		assert.EqualValues("test", recorder.Header().Get("X-test"))
	})

	//Response is not msgpack-encoded
	t.Run("UnexpectedResponseFormat", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		response := &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewBufferString("t")),
		}

		assert.NotNil(encodeResponse(context.TODO(), recorder, response))
	})

	//XMiDt gives us a 200 (OK) with a well-formatted RDK device response
	t.Run("RDKDeviceResponse", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		response := &http.Response{
			StatusCode: http.StatusOK,
			Body: ioutil.NopCloser(bytes.NewBuffer(wrp.MustEncode(&wrp.Message{
				Type:    wrp.SimpleRequestResponseMessageType,
				Payload: []byte(`{"statusCode": 520}`),
			}, wrp.Msgpack))),
		}

		err := encodeResponse(context.TODO(), recorder, response)
		assert.Nil(err)
		assert.EqualValues(520, recorder.Code)
		assert.EqualValues(`{"statusCode": 520}`, recorder.Body.String())
	})

	//If the RDK device is having an internal error, avoid ambiguity confusion by not returning 500 (Tr1d1um is not the one having
	//an internal error)
	t.Run("InternalRDKDeviceError", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		internalErrorResponse := []byte(`{"statusCode": 500, "message": "I, the device, suffer`)

		response := &http.Response{
			StatusCode: http.StatusOK,
			Body: ioutil.NopCloser(bytes.NewBuffer(wrp.MustEncode(&wrp.Message{
				Type:    wrp.SimpleRequestResponseMessageType,
				Payload: internalErrorResponse}, wrp.Msgpack))),
		}

		err := encodeResponse(context.TODO(), recorder, response)
		assert.Nil(err)
		assert.EqualValues(http.StatusOK, recorder.Code)
		assert.EqualValues(internalErrorResponse, recorder.Body.Bytes())
	})

	//For whatever reason, the device may respond with incomplete or unexpected data
	t.Run("BadRDKDeviceResponse", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		response := &http.Response{
			StatusCode: http.StatusOK,
			Body: ioutil.NopCloser(bytes.NewBuffer(wrp.MustEncode(&wrp.Message{
				Type:    wrp.SimpleRequestResponseMessageType,
				Payload: []byte(`{"statusCode":`),
			}, wrp.Msgpack))),
		}

		err := encodeResponse(context.TODO(), recorder, response)
		assert.Nil(err)
		assert.EqualValues(http.StatusOK, recorder.Code)
		assert.EqualValues(`{"statusCode":`, recorder.Body.String())
	})
}