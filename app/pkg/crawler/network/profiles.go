package network

import (
	"fmt"
	"strings"
)

// Profile represents a set of HTTP headers consistent with a TLS Client Hello ID
type Profile struct {
	TLSClientHelloID *ClientHelloID
	Headers          Headers
}

func (p Profile) GetFullHeaders() map[string]string {
	fullHeaders := map[string]string{
		"User-Agent":                p.Headers.UserAgent,
		"Accept-Language":           p.Headers.AcceptLanguage,
		"Accept-Encoding":           p.Headers.AcceptEncoding,
		"Accept":                    p.Headers.Accept,
		"Referer":                   p.Headers.Referer,
		"Referrer-Policy":           p.Headers.ReferrerPolicy,
		"Connection":                "close",
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Dest":            "document",
		"Cache-Control":             "max-age=0",
		"TE":                        "trailers",
		"Pragma":                    "no-cache",
	}

	if p.Headers.Implements.DeviceMemory {
		fullHeaders["Device-Memory"] = p.Headers.DeviceMemory
	}
	if p.Headers.Implements.SecFetchUser {
		fullHeaders["Sec-Fetch-User"] = "?1"
	}
	if p.Headers.Implements.SecCh {
		fullHeaders["Sec-Ch-UA"] = p.Headers.SecChUa
		fullHeaders["Sec-Ch-UA-Mobile"] = p.Headers.SecChUaMobile
		fullHeaders["Sec-Ch-UA-Platform"] = p.Headers.SecChUaPlatform
	}
	if p.Headers.SecChUaMobile == "?1" {
		fullHeaders["Viewport-Width"] = p.Headers.ViewportWidth
	}

	return fullHeaders
}

type internalHeaders struct {
	userAgent      string
	referrer       string
	acceptLanguage string
	acceptEncoding string
	referrerPolicy string
	deviceMemory   string
	viewportWidth  string
}

type internalBrowserData struct {
	browser      string
	majorVersion string
	os           string
	mobile       string
}

// generateProfiles creates multiple consistent browser profiles.
//
// In the cases of a TLS Client Hello ID that does not implement certain headers,
// multiple equal profiles will be generated, but the extremely high amount of
// generated profiles for each ID will ensure that the distribution is good enough.
func generateProfiles(userAgents []string) []*Profile {
	profiles := []*Profile{}

	// TODO: Maybe implement this as a cartesian product?
	for _, userAgent := range userAgents {
		browser, majorVersion, os, mobile := detectBrowserInfo(userAgent)

		iHeaders := &internalHeaders{
			userAgent: userAgent,
		}

		iBrowserData := &internalBrowserData{
			browser:      browser,
			majorVersion: majorVersion,
			os:           os,
			mobile:       mobile,
		}

		for _, referrer := range referrers {
			iHeaders.referrer = referrer

			for _, acceptLanguage := range acceptLanguages {
				iHeaders.acceptLanguage = acceptLanguage

				for _, acceptEncoding := range acceptEncodings {
					iHeaders.acceptEncoding = acceptEncoding

					for _, referrerPolicy := range referrerPolicies {
						iHeaders.referrerPolicy = referrerPolicy

						for _, deviceMemory := range deviceMemoryValues {
							iHeaders.deviceMemory = deviceMemory

							for _, viewportWidth := range viewportWidths {
								iHeaders.viewportWidth = viewportWidth

								profile := generateProfile(
									iHeaders,
									iBrowserData,
								)

								profiles = append(profiles, profile)
							}
						}
					}
				}
			}
		}
	}

	return profiles
}

// generateProfile creates a logically consistent set of HTTP headers along with a valid TLS Client Hello ID
func generateProfile(headers *internalHeaders, browserData *internalBrowserData) *Profile {
	// Generate matching sec-ch-ua
	secChUa := generateMatchingSecChUa(browserData)

	// Set platform based on OS
	secChUaPlatform := fmt.Sprintf("\"%s\"", browserData.os)

	// Select appropriate Accept header based on browser
	accept := generateLogicalAccept(browserData.browser)

	// Pick a logical TLS profile based on browser
	tlsProfile := pickLogicalTLSProfile(browserData)

	isNotFirefox := browserData.browser != "Firefox"
	isNotSafari := browserData.browser != "Safari"

	acceptEncoding := headers.acceptEncoding
	if !isNotSafari {
		acceptEncoding = acceptEncodings[0] // Safari does not support zstd
	}

	return &Profile{
		TLSClientHelloID: tlsProfile,
		Headers: Headers{
			UserAgent:       headers.userAgent,
			Referer:         headers.referrer,
			AcceptLanguage:  headers.acceptLanguage,
			AcceptEncoding:  acceptEncoding,
			Accept:          accept,
			SecChUa:         secChUa,
			SecChUaMobile:   browserData.mobile,
			SecChUaPlatform: secChUaPlatform,
			DeviceMemory:    headers.deviceMemory,
			ViewportWidth:   headers.viewportWidth,
			ReferrerPolicy:  headers.referrerPolicy,
			Implements: implements{
				DeviceMemory: isNotFirefox && isNotSafari,
				SecFetchUser: isNotFirefox && isNotSafari,
				SecCh:        isNotFirefox,
			},
		},
	}
}

// detectBrowserInfo extracts browser, OS, and device information from a user agent
func detectBrowserInfo(userAgent string) (browser, majorVersion, os, mobile string) {
	// Default values
	browser = "Chrome"
	majorVersion = "0"
	os = "Windows"
	mobile = "?0" // Default to desktop

	isAndroid := strings.Contains(userAgent, "Android")

	// Check for mobile
	if strings.Contains(userAgent, "Mobile") || isAndroid {
		mobile = "?1"
	}

	// Detect OS
	switch {
	case isAndroid:
		os = "Android"
	case strings.Contains(userAgent, "Windows"):
		os = "Windows"
	case strings.Contains(userAgent, "Linux"):
		os = "Linux"
	case strings.Contains(userAgent, "iPhone"):
		os = "iOS"
	case strings.Contains(userAgent, "Macintosh") || strings.Contains(userAgent, "Mac OS X"):
		os = "macOS"
	case strings.Contains(userAgent, "iPad"):
		os = "iPadOS"
	}

	isOPR := strings.Contains(userAgent, "OPR/")
	isOpera := strings.Contains(userAgent, "Opera/")

	// Detect the browser and its version
	var parts []string
	switch {
	case strings.Contains(userAgent, "Firefox/"):
		browser = "Firefox"
		parts = strings.Split(userAgent, "Firefox/")
	case strings.Contains(userAgent, "Edg/"):
		browser = "Edge"
		parts = strings.Split(userAgent, "Edg/")
	case strings.Contains(userAgent, "Chrome/"):
		browser = "Chrome" // override Opera and SamsungBrowser UAs with Chrome until we support them (implement the fingerprints)
		parts = strings.Split(userAgent, "Chrome/")
	case strings.Contains(userAgent, "Safari/"):
		browser = "Safari"
		parts = strings.Split(userAgent, "Version/")
	case isOPR || isOpera:
		browser = "Opera"
		if isOPR {
			parts = strings.Split(userAgent, "OPR/")
		} else {
			parts = strings.Split(userAgent, "Version/")
		}
	case strings.Contains(userAgent, "SamsungBrowser/"):
		browser = "Samsung Browser"
		parts = strings.Split(userAgent, "SamsungBrowser/")
	default:
		parts = strings.Split(userAgent, "Version/")
	}

	versionPart := parts[len(parts)-1]
	majorVersion = strings.TrimSpace(strings.Split(versionPart, ".")[0])

	return browser, majorVersion, os, mobile
}

// generateMatchingSecChUa creates a matching sec-ch-ua header based on the browser info
func generateMatchingSecChUa(browserData *internalBrowserData) string {
	majorVersion := browserData.majorVersion

	switch browserData.browser {
	case "Chrome":
		return fmt.Sprintf("\"Google Chrome\";v=\"%s\", \"Chromium\";v=\"%s\", \"Not:A-Brand\";v=\"99\"", majorVersion, majorVersion)
	case "Edge":
		return fmt.Sprintf("\"Microsoft Edge\";v=\"%s\", \"Chromium\";v=\"%s\", \"Not:A-Brand\";v=\"99\"", majorVersion, majorVersion)
	case "Safari":
		if browserData.os == "iPhone" {
			return fmt.Sprintf("\"Mobile Safari\";v=\"%s\", \"Safari\";v=\"%s\"", majorVersion, majorVersion)
		}
		return fmt.Sprintf("\"Safari\";v=\"%s\", \"Not:A-Brand\";v=\"99\"", majorVersion)
	default:
		// Fallback for unknown browsers
		return ""
	}
}

// generateLogicalAccept returns an appropriate Accept header based on the browser
func generateLogicalAccept(browser string) string {
	switch browser {
	case "Firefox":
		return "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"
	case "Safari":
		return "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"
	default:
		return "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"
	}
}

func pickLogicalTLSProfile(browserData *internalBrowserData) *ClientHelloID {
	if browserHelloIDs, ok := clientHelloIDs[browserData.browser]; ok {
		if clientHelloID, ok := browserHelloIDs[browserData.majorVersion]; ok {
			return clientHelloID
		}
	}

	// Fallback to Chrome 102 as it is the most adapative
	return clientHelloIDs["Chrome"]["102"]
}
