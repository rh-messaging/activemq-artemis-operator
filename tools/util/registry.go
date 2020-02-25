package util

import (
	"encoding/json"
	"fmt"
	"github.com/heroku/docker-registry-client/registry"
	"github.com/rh-messaging/activemq-artemis-operator/tools/constants"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

type QuayTag struct {
	Revision       bool   `json:"revision"`
	StartTs        int    `json:"start_ts"`
	Name           string `json:"name"`
	DockerImageID  string `json:"docker_image_id"`
	ImageID        string `json:"image_id"`
	ManifestDigest string `json:"manifest_digest"`
	Modified       string `json:"last_modified"`
	IsManifestList bool   `json:"is_manifest_list"`
	Size           int64  `json:"size"`
	Reversion      bool   `json:"reversion"`
	StartTS        int64  `json:"start_ts"`
}

type QuayTagsResponse struct {
	HasAdditional bool      `json:"has_additional"`
	Page          int       `json:"page"`
	Tags          []QuayTag `json:"tags"`
}

func constructQuayURL(image string, imageTag string) (string, error) {
	u, err := url.Parse(constants.QuayURLBase)
	if err != nil {
		return "", err
	}

	u.Path = path.Join(u.Path, image, "tag") + "/"

	q := u.Query()
	q.Add("onlyActiveTags", "true")
	q.Add("specificTag", imageTag)
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func RetriveFromQuayIO(image string, imageTag string) ([]string, error) {
	url, err := constructQuayURL(image, imageTag)

	if err != nil {
		return nil, err
	}
	body, err := httpGet(url, os.Getenv("QUAYIO_TOKEN"), nil)
	if err != nil {
		return nil, err
	}

	var resp QuayTagsResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, err
	}

	sha := []string{}

	for _, tag := range resp.Tags {
		sha = append(sha, tag.ManifestDigest)
	}

	return sha, err
}

type BasicAuthInfo struct {
	Username string
	Password string
}

func httpGet(url, apiToken string, basicAuth *BasicAuthInfo) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+apiToken)
	} else if basicAuth != nil {
		req.SetBasicAuth(basicAuth.Username, basicAuth.Password)
	} else {
		return "", fmt.Errorf("No apiToken and Basic Auth provided for registry, Please add QUAYIO_TOKEN ", url)
	}

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP error!\nURL: %s\nstatus code: %d\nbody:\n%s\n", url, resp.StatusCode, string(b))
	}

	return string(b), err
}

func RetriveFromRedHatIO(image string, imageTag string) ([]string, error) {

	url := "https://" + constants.RedHatImageRegistry

	username := "" // anonymous
	password := "" // anonymous

	if userToken := strings.Split(os.Getenv("REDHATIO_TOKEN"), ":"); len(userToken) > 1 {
		username = userToken[0]
		password = userToken[1]
	}
	hub, err := registry.New(url, username, password)
	digests := []string{}
	if err != nil {
		fmt.Println(err)
	} else {
		tags, err := hub.Tags(image)
		if err != nil {
			fmt.Println(err)
		}
		// do not follow redirects - this is critical so we can get the registry digest from Location in redirect response
		hub.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		if _, exists := find(tags, imageTag); exists {
			req, err := http.NewRequest("GET", url+"/v2/"+image+"/manifests/"+imageTag, nil)
			if err != nil {
				fmt.Println(err)
			}
			req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
			resp, err := hub.Client.Do(req)
			if err != nil {
				fmt.Println(err)
			}
			if resp != nil {
				defer resp.Body.Close()
			}
			if resp.StatusCode == 302 || resp.StatusCode == 301 {
				digestURL, err := resp.Location()
				if err != nil {
					fmt.Println(err)

				}

				if digestURL != nil {
					if url := strings.Split(digestURL.EscapedPath(), "/"); len(url) > 1 {
						digests = url

						return url, err
					}
				}
			}
		}

	}
	return digests, err
}

func find(slice []string, val string) (int, bool) {
	for i, item := range slice {
		if item == val {
			return i, true
		}
	}
	return -1, false
}
