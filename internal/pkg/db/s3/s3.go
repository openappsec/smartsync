// Copyright (C) 2022 Check Point Software Technologies Ltd. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package s3repository

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/xml"
	errs "errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"openappsec.io/smartsync-service/models"
	"time"

	"openappsec.io/ctxutils"
	"openappsec.io/errors"
	"openappsec.io/httputils/client"
	"openappsec.io/log"
)

const (
	traceClientTimeout = 2 * time.Minute

	//conf Key Reverse Proxy
	rp                   = "rp"
	rpBaseURL            = rp + ".baseUrl"
	sharedStorageKey     = "shared_storage"
	sharedStorageHostKey = sharedStorageKey + ".host"

	tenantIDHeader         = "x-tenant-id"
	headerKeyTraceID       = "X-Trace-Id"
	headerKeyCorrelationID = "X-Correlation-Id"

	maxRetries = 3
)

// Configuration exposes an interface of configuration related actions
type Configuration interface {
	GetString(key string) (string, error)
}

// Adapter to intelligence DB sdk
type Adapter struct {
	HTTPClient *http.Client
	baseURL    string
}

// NewAdapter creates new adapter
func NewAdapter(c Configuration) (*Adapter, error) {
	a := &Adapter{}
	if err := a.initialize(c); err != nil {
		return &Adapter{}, errors.Wrap(err, "failed to initialize S3 rp adapter")
	}
	return a, nil
}

// initialize the adapter
func (a *Adapter) initialize(c Configuration) error {
	var baseURL string
	host, err := c.GetString(sharedStorageHostKey)
	if err != nil {
		baseURL, err = c.GetString(rpBaseURL)
	} else {
		baseURL = fmt.Sprintf("http://%s/api", host)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to get reverse proxy baseURL from %v", rpBaseURL)
	}
	if _, err = url.Parse(baseURL); err != nil {
		return err
	}
	a.baseURL = baseURL
	a.HTTPClient = client.NewTracerClient(traceClientTimeout)

	return nil
}

// GetFileRaw - download file, return raw data and true if data was decompressed
func (a *Adapter) GetFileRaw(ctx context.Context, tenantID string, path string) ([]byte, bool, error) {
	if path[:1] != "/" {
		path = "/" + path
	}
	path = a.baseURL + path
	resp, err := a.getFile(ctx, tenantID, path)
	if err != nil {
		return []byte{}, false, errors.Wrapf(err, "failed to get file %v", path)
	}
	if resp == nil || len(resp) == 0 {
		return []byte{}, false, errors.Errorf("got empty file: %v", path)
	}

	//might be compressed
	if !json.Valid(resp) {
		b := bytes.NewReader(resp)
		compressor, err := gzip.NewReader(b)
		if err != nil {
			log.WithContext(ctx).Warnf("failed to create reader, err: %v", err)
			return []byte{}, false, errors.Wrapf(err, "failed to create reader %v", string(resp))
		}
		decompressed, err := ioutil.ReadAll(compressor)
		defer compressor.Close()
		if err != nil && !errs.Is(err, io.ErrUnexpectedEOF) {
			log.WithContext(ctx).Warnf("failed to decompress, err: %v, is EOF: %v", err, err == io.EOF)
			return []byte{}, false, errors.Wrapf(err, "failed to decompress %v", string(resp))
		}
		log.WithContext(ctx).Infof(
			"decompress ok, compression ratio %v", float64(len(decompressed))/float64(len(resp)),
		)
		return decompressed, true, nil
	}
	log.WithContext(ctx).Debugf("got file: %v (uncompressed)", path)
	return resp, false, nil
}

// GetFile - download file and unmarshal to out return true is data was decompressed
func (a *Adapter) GetFile(ctx context.Context, tenantID string, path string, out interface{}) (bool, error) {
	data, isCompressed, err := a.GetFileRaw(ctx, tenantID, path)
	if err != nil {
		return isCompressed, err
	}

	err = json.Unmarshal(data, out)
	if err != nil {
		return isCompressed, errors.Wrapf(err, "failed to unmarshal %v", string(data))
	}
	log.WithContext(ctx).Debugf("got file: %v, post unmarshal: %v", path, out)
	return isCompressed, nil
}

func (a *Adapter) getFile(ctx context.Context, tenantID string, path string) ([]byte, error) {
	u, err := url.Parse(path)
	if err != nil {
		return []byte{}, err
	}
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return []byte{}, errors.Wrapf(
			err, "failed to generate new request to get decisions from s3 bucket %v", u.String(),
		)
	}
	req.Header.Add(tenantIDHeader, tenantID)
	req.Header.Set(headerKeyTraceID, ctxutils.ExtractString(ctx, ctxutils.ContextKeyEventTraceID))
	req.Header.Set(headerKeyCorrelationID, ctxutils.ExtractString(ctx, ctxutils.ContextKeyEventTraceID))

	resp, err := a.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return []byte{}, errors.Wrapf(err, "failed to get from %v", u.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return []byte{}, errors.Errorf("file %v not found", path).SetClass(errors.ClassNotFound)
		}
		return []byte{}, errors.Errorf("get %v failed, status: %v", u.String(), resp.Status)
	}

	resBody, bodyErr := ioutil.ReadAll(resp.Body)
	if bodyErr != nil {
		return []byte{}, errors.Errorf("read body from get file resp failed: %v", u.String(), bodyErr)
	}
	return resBody, nil
}

// PostFile - marshal data and put file
func (a *Adapter) PostFile(ctx context.Context, tenantID string, path string, compress bool, data interface{}) error {
	path = a.baseURL + path

	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")
	err := enc.Encode(data)
	if err != nil {
		return errors.Wrap(err, "Failed to encode")
	}

	bufTrimmed := bytes.TrimRight(buf.Bytes(), "\n")

	if compress {
		var b bytes.Buffer
		compressor := gzip.NewWriter(&b)
		_, err = compressor.Write(bufTrimmed)
		if err != nil {
			log.WithContext(ctx).Warnf("failed to compress marshaled json, err: %v", err)
		} else {
			err = compressor.Flush()
			if err != nil {
				return err
			}
			err = compressor.Close()
			if err != nil {
				return err
			}
			bufTrimmed = b.Bytes()
		}
	}

	return a.putFileWithRetry(ctx, bufTrimmed, path, tenantID)
}

func (a *Adapter) putFileWithRetry(ctx context.Context, body []byte, path string, tenantID string) error {
	u, err := url.Parse(path)
	if err != nil {
		return err
	}

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Second*time.Duration(i) + time.Millisecond*time.Duration(rand.Int()%1000))
			log.WithContext(ctx).Infof("retry request to %v", path)
		}
		requestReader := bytes.NewReader(body)
		req, err2 := http.NewRequest(http.MethodPut, path, requestReader)
		if err2 != nil {
			return errors.Wrapf(err2, "failed to generate new request to put file in s3 bucket %v", u.String())
		}
		req.Header.Add(tenantIDHeader, tenantID)
		req.Header.Set(headerKeyTraceID, ctxutils.ExtractString(ctx, ctxutils.ContextKeyEventTraceID))
		req.Header.Set(headerKeyCorrelationID, ctxutils.ExtractString(ctx, ctxutils.ContextKeyEventTraceID))

		resp, err2 := a.HTTPClient.Do(req.WithContext(ctx))
		if err2 != nil {
			return errors.Wrapf(err2, "failed to put file to %v", u.String())
		}

		if resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}

		resBody, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		err = errors.Errorf("Put %v failed, status: %v, body: %s", u.String(), resp.Status, resBody)
		log.WithContext(ctx).Warnf("attempt: %v returned error: %v", i, err)
	}
	return err
}

// GetFilesList return a list of files with common prefix
func (a *Adapter) GetFilesList(ctx context.Context, id models.SyncID) ([]string, error) {
	var path string
	if len(id.Type) > 0 {
		if len(id.WindowID) > 0 {
			path = fmt.Sprintf("%v/?list-type=2&prefix=%v/%v/%v/%v/", a.baseURL, id.TenantID, id.AssetID, id.Type, id.WindowID)
		} else {
			path = fmt.Sprintf("%v/?list-type=2&prefix=%v/%v/%v/", a.baseURL, id.TenantID, id.AssetID, id.Type)
		}
	} else {
		path = fmt.Sprintf("%v/?list-type=2&prefix=%v/%v/", a.baseURL, id.TenantID, id.AssetID)
	}
	u, err := url.Parse(path)
	if err != nil {
		return []string{}, err
	}
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return []string{}, errors.Wrapf(
			err, "failed to generate new request to get decisions from s3 bucket %v", u.String(),
		)
	}
	req.Header.Add(tenantIDHeader, id.TenantID)
	req.Header.Set(headerKeyTraceID, ctxutils.ExtractString(ctx, ctxutils.ContextKeyEventTraceID))
	req.Header.Set(headerKeyCorrelationID, ctxutils.ExtractString(ctx, ctxutils.ContextKeyEventTraceID))

	resp, err := a.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return []string{}, errors.Wrapf(err, "failed to get from %v", u.String())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return []string{}, errors.Errorf("file %v not found", path).SetClass(errors.ClassNotFound)
		}
		return []string{}, errors.Errorf("get %v failed, status: %v", u.String(), resp.Status)
	}

	resBody, bodyErr := ioutil.ReadAll(resp.Body)
	if bodyErr != nil {
		return []string{}, errors.Errorf("read body from get file resp failed: %v", u.String(), bodyErr)
	}

	return extractFilesList(resBody)
}

type owner struct {
	DisplayName string `xml:"DisplayName"`
	ID          string `xml:"ID"`
}

type contents struct {
	ETag         string `xml:"ETag"`
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	Owner        owner  `xml:"Owner"`
	Size         string `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

type commonPrefixes struct {
	Prefix string `xml:"Prefix"`
}

type listBucketResult struct {
	XMLName               xml.Name         `xml:"ListBucketResult"`
	IsTruncated           bool             `xml:"IsTruncated"`
	Contents              []contents       `xml:"Contents"`
	Name                  string           `xml:"Name"`
	Prefix                string           `xml:"Prefix"`
	Delimiter             string           `xml:"Delimiter"`
	MaxKeys               string           `xml:"MaxKeys"`
	CommonPrefixes        []commonPrefixes `xml:"CommonPrefixes"`
	EncodingType          string           `xml:"EncodingType"`
	KeyCount              string           `xml:"KeyCount"`
	ContinuationToken     string           `xml:"ContinuationToken"`
	NextContinuationToken string           `xml:"NextContinuationToken"`
	StartAfter            string           `xml:"StartAfter"`
}

func extractFilesList(body []byte) ([]string, error) {
	var listRes listBucketResult
	err := xml.Unmarshal(body, &listRes)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, content := range listRes.Contents {
		ids = append(ids, content.Key)
	}
	return ids, nil
}
