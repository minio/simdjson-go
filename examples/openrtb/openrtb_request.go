package main

import (
	"fmt"
	"github.com/minio/simdjson-go"
	"os"
	"sync"
)

var tagParamsPool sync.Pool

// TagParams Extract only basic needed fields from the OpenRTB request
type TagParams struct {
	ReqId        string
	ImpressionId string
	SiteUrl      string
	PublisherId  string
	Referrer     string
	UserId       string
	AppId        string
	AppName      string
	AppStoreUrl  string
	SiteDomain   string
	SiteId       string
	Height       string
	Width        string
	Ip           string
	DeviceType   uint8
	Carrier      string
	DeviceUa     string
	DeviceMake   string
	DeviceId     string
	OS           string
	Longitude    string
	Latitude     string
	Country      string
	City         string
	State        string
	Metro        string
	Postal       string
	Bcat         []string
	TimeMax      int16
	BidFloor     float32
	ParsedBid    *simdjson.ParsedJson
}

func (tp *TagParams) Reset() {
	parsedBid := tp.ParsedBid
	// re-init tp. Go compiler optimizes it. Note that it will all inner structs will be cleared too
	*tp = TagParams{}
	// The ParsedBid was a pointer but it was set to nil after the reset, so we must restore it
	tp.ParsedBid = parsedBid
}

func populateTagParams(bidReqBody []byte) (*TagParams, error) {
	tagParams := AcquireTagParams()
	// Reuse previously used ParsedBid and don't copy field
	bidReq, err := simdjson.Parse(bidReqBody, tagParams.ParsedBid, simdjson.WithCopyStrings(false))
	if err != nil {
		ReleaseTagParams(tagParams)
		return nil, err
	}
	// store the ParsedJson for reuse next time
	tagParams.ParsedBid = bidReq

	bidReqId := ""
	tmax := int64(0)
	bcat := []string(nil)
	userId := ""
	siteId := ""
	siteDomain := ""
	sitePage := ""
	siteReferrer := ""
	publisherId := ""
	appBundle := ""
	appId := ""
	appName := ""
	appStoreUrl := ""
	ipV4 := ""
	ipV6 := ""
	deviceType := int64(0)
	deviceOs := ""
	deviceMake := ""
	deviceIfa := ""
	deviceCarrier := ""
	deviceUa := ""
	country := ""
	longitude := ""
	latitude := ""
	city := ""
	state := ""
	metro := ""
	postal := ""
	impId := ""
	bidFloor := float64(0)
	width := ""
	height := ""

	bidReqIter := bidReq.Iter()
	typ := bidReqIter.Advance()
	if typ != simdjson.TypeRoot {
		return nil, err
	}
	rootIter := &simdjson.Iter{}
	typ, _, err = bidReqIter.Root(rootIter)
	if err != nil || typ != simdjson.TypeObject {
		return nil, err
	}
	rootObj := &simdjson.Object{}
	_, err = rootIter.Object(rootObj)
	if err != nil {
		return nil, err
	}
	l1PropsIter := &simdjson.Iter{}
	for {
		l1PropName, l1PropType, l1PropErr := rootObj.NextElementBytes(l1PropsIter)
		if l1PropType == simdjson.TypeNone || l1PropErr != nil {
			break
		}
		switch string(l1PropName) {
		case "id":
			bidReqId, _ = l1PropsIter.String()
		case "tmax":
			tmax, _ = l1PropsIter.Int()
		case "bcat":
			arr := &simdjson.Array{}
			_, err = l1PropsIter.Array(arr)
			if err != nil {
				continue
			}
			bcat, _ = arr.AsString()
		case "imp":
			arr := &simdjson.Array{}
			_, err = l1PropsIter.Array(arr)
			if err != nil {
				continue
			}
			impIter := arr.Iter()
			impType := impIter.Advance()
			if impType != simdjson.TypeObject {
				continue
			}
			impObj := &simdjson.Object{}
			_, err = impIter.Object(impObj)
			if err != nil {
				continue
			}
			impObjPropsIter := &simdjson.Iter{}
			for {
				impPropName, impPropType, impPropErr := impObj.NextElementBytes(impObjPropsIter)
				if impPropType == simdjson.TypeNone || impPropErr != nil {
					break
				}
				switch string(impPropName) {
				case "id":
					impId, _ = impObjPropsIter.String()
				case "bidfloor":
					bidFloor, _ = impObjPropsIter.Float()
				case "banner":
					bannerObj := &simdjson.Object{}
					_, err = impObjPropsIter.Object(bannerObj)
					if err != nil {
						continue
					}
					bannerPropsIter := &simdjson.Iter{}
					for {
						bannerPropName, bannerPropType, bannerPropErr := bannerObj.NextElementBytes(bannerPropsIter)
						if bannerPropType == simdjson.TypeNone || bannerPropErr != nil {
							break
						}
						switch string(bannerPropName) {
						case "w":
							// convert int to str
							width, _ = bannerPropsIter.StringCvt()
						case "h":
							// convert int to str
							height, _ = bannerPropsIter.StringCvt()
						}
					}
				case "video":
					videoObj := &simdjson.Object{}
					_, err = impObjPropsIter.Object(videoObj)
					if err != nil {
						continue
					}
					videoPropsIter := &simdjson.Iter{}
					for {
						videoPropName, videoPropType, videoPropErr := videoObj.NextElementBytes(videoPropsIter)
						if videoPropType == simdjson.TypeNone || videoPropErr != nil {
							break
						}
						switch string(videoPropName) {
						case "w":
							// convert int to str
							width, _ = videoPropsIter.StringCvt()
						case "h":
							// convert int to str
							height, _ = videoPropsIter.StringCvt()
						}
					}
				}
			}
		case "user":
			userObj := &simdjson.Object{}
			_, err = l1PropsIter.Object(userObj)
			if err != nil {
				continue
			}
			userObjPropsIter := &simdjson.Iter{}
			for {
				userPropName, userPropType, userPropErr := userObj.NextElementBytes(userObjPropsIter)
				if userPropType == simdjson.TypeNone || userPropErr != nil {
					break
				}
				switch string(userPropName) {
				case "id":
					userId, _ = userObjPropsIter.String()
				}
			}
		case "app":
			appObj := &simdjson.Object{}
			_, err = l1PropsIter.Object(appObj)
			if err != nil {
				continue
			}
			appObjPropsIter := &simdjson.Iter{}
			for {
				appPropName, appPropType, appPropErr := appObj.NextElementBytes(appObjPropsIter)
				if appPropType == simdjson.TypeNone || appPropErr != nil {
					break
				}
				switch string(appPropName) {
				case "id":
					appId, _ = appObjPropsIter.String()
				case "bundle":
					appBundle, _ = appObjPropsIter.String()
				case "name":
					appName, _ = appObjPropsIter.String()
				case "storeurl":
					appStoreUrl, _ = appObjPropsIter.String()
				case "publisher":
					appPublisherObj := &simdjson.Object{}
					_, err = appObjPropsIter.Object(appPublisherObj)
					if err != nil {
						continue
					}
					appPublisherPropsIter := &simdjson.Iter{}
					for {
						appPublisherPropName, appPublisherPropType, appPublisherPropErr := appPublisherObj.NextElementBytes(appPublisherPropsIter)
						if appPublisherPropType == simdjson.TypeNone || appPublisherPropErr != nil {
							break
						}
						switch string(appPublisherPropName) {
						case "id":
							publisherId, _ = appPublisherPropsIter.String()
						}
					}
				}
			}
		case "site":
			siteObj := &simdjson.Object{}
			_, err = l1PropsIter.Object(siteObj)
			if err != nil {
				continue
			}
			siteObjPropsIter := &simdjson.Iter{}
			for {
				sitePropName, sitePropType, sitePropErr := siteObj.NextElementBytes(siteObjPropsIter)
				if sitePropType == simdjson.TypeNone || sitePropErr != nil {
					break
				}
				switch string(sitePropName) {
				case "id":
					siteId, _ = siteObjPropsIter.String()
				case "domain":
					siteDomain, _ = siteObjPropsIter.String()
				case "page":
					sitePage, _ = siteObjPropsIter.String()
				case "ref":
					siteReferrer, _ = siteObjPropsIter.String()
				case "publisher":
					sitePublisherObj := &simdjson.Object{}
					_, err = siteObjPropsIter.Object(sitePublisherObj)
					if err != nil {
						continue
					}
					sitePublisherPropsIter := &simdjson.Iter{}
					for {
						sitePublisherPropName, sitePublisherPropType, sitePublisherPropErr := sitePublisherObj.NextElementBytes(sitePublisherPropsIter)
						if sitePublisherPropType == simdjson.TypeNone || sitePublisherPropErr != nil {
							break
						}
						switch string(sitePublisherPropName) {
						case "id":
							publisherId, _ = sitePublisherPropsIter.String()
						}
					}
				}
			}
		case "device":
			deviceObj := &simdjson.Object{}
			_, err = l1PropsIter.Object(deviceObj)
			if err != nil {
				continue
			}
			deviceObjPropsIter := &simdjson.Iter{}
			for {
				devicePropName, devicePropType, devicePropErr := deviceObj.NextElementBytes(deviceObjPropsIter)
				if devicePropType == simdjson.TypeNone || devicePropErr != nil {
					break
				}
				switch string(devicePropName) {
				case "ip":
					ipV4, _ = deviceObjPropsIter.String()
				case "ipv6":
					ipV6, _ = deviceObjPropsIter.String()
				case "carrier":
					deviceCarrier, _ = deviceObjPropsIter.String()
				case "ua":
					deviceUa, _ = deviceObjPropsIter.String()
				case "os":
					deviceOs, _ = deviceObjPropsIter.String()
				case "ifa":
					deviceIfa, _ = deviceObjPropsIter.String()
				case "make":
					deviceMake, _ = deviceObjPropsIter.String()
				case "devicetype":
					deviceType, _ = deviceObjPropsIter.Int()
				case "geo":
					geoObj := &simdjson.Object{}
					_, err = deviceObjPropsIter.Object(geoObj)
					if err != nil {
						continue
					}
					geoPropsIter := &simdjson.Iter{}
					for {
						geoPropName, geoPropType, geoPropErr := geoObj.NextElementBytes(geoPropsIter)
						if geoPropType == simdjson.TypeNone || geoPropErr != nil {
							break
						}
						switch string(geoPropName) {
						case "country":
							country, _ = geoPropsIter.String()
						case "city":
							city, _ = geoPropsIter.String()
						case "region":
							state, _ = geoPropsIter.String()
						case "metro":
							metro, _ = geoPropsIter.String()
						case "zip":
							postal, _ = geoPropsIter.String()
						case "lon":
							// convert float to str
							longitude, _ = geoPropsIter.StringCvt()
						case "lat":
							// convert float to str
							latitude, _ = geoPropsIter.StringCvt()
						}
					}
				}
			}
		}
	}

	fillTagParams(tagParams, ipV4, ipV6, bidReqId, impId, sitePage, siteReferrer, deviceUa, userId, appBundle, publisherId, appId, siteDomain, siteId, width, height, country, city, state, metro, postal, deviceOs, deviceMake, deviceIfa, longitude, latitude, deviceCarrier, appName, appStoreUrl, tmax, bidFloor, deviceType, bcat)
	return tagParams, nil
}

func fillTagParams(tagParams *TagParams, ipV4, ipV6, bidReqId, impId, sitePage, siteReferrer, deviceUa, userId, appBundle, publisherId, appId, siteDomain, siteId, width, height, country, city, state, metro, postal, deviceOs, deviceMake, deviceIfa, longitude, latitude, deviceCarrier, appName, appStoreUrl string, tmax int64, bidFloor float64, deviceType int64, bcats []string) {
	ip := ""
	if ipV6 != "" {
		ip = ipV6
	} else if ipV4 != "" {
		ip = ipV4
	}

	tagParams.ReqId = bidReqId
	tagParams.ImpressionId = impId
	tagParams.SiteUrl = sitePage
	tagParams.Referrer = siteReferrer
	tagParams.DeviceUa = deviceUa
	tagParams.Ip = ip
	tagParams.UserId = userId
	tagParams.AppId = appBundle
	tagParams.PublisherId = publisherId
	tagParams.DeviceType = uint8(deviceType)
	tagParams.AppId = appId
	tagParams.SiteDomain = siteDomain
	tagParams.SiteId = siteId
	tagParams.Width = width
	tagParams.Height = height
	tagParams.Country = country
	tagParams.City = city
	tagParams.State = state
	tagParams.Metro = metro
	tagParams.Postal = postal
	tagParams.OS = deviceOs
	tagParams.DeviceMake = deviceMake
	tagParams.DeviceId = deviceIfa
	tagParams.Longitude = longitude
	tagParams.Latitude = latitude
	tagParams.Ip = ip
	tagParams.Carrier = deviceCarrier
	tagParams.Bcat = bcats
	tagParams.AppName = appName
	tagParams.AppStoreUrl = appStoreUrl
	tagParams.TimeMax = int16(tmax)
	tagParams.BidFloor = float32(bidFloor)
}

func AcquireTagParams() *TagParams {
	var tagParams *TagParams
	oldTagParams := tagParamsPool.Get()
	if oldTagParams != nil {
		tagParams = oldTagParams.(*TagParams)
	} else {
		tagParams = &TagParams{}
	}
	return tagParams
}

func ReleaseTagParams(tagParams *TagParams) {
	tagParams.Reset()
	tagParamsPool.Put(tagParams)
}

var reqBody = []byte(`{"id":"banner1","at":2,"tmax":300,"cur":["USD"],"imp":[{"id":"imp1","secure":0,"video":{"w":1920,"h":1080,"mimes":["video/mp4"],"protocols":[1,2,3,4,5,6],"linearity":1,"minduration":3,"maxduration":300,"api":[1,2],"boxingallowed":1},"bidfloor":1.25}],"device":{"dnt":0,"devicetype":3,"geo":{"country":"USA","type":2,"lat":40.9295,"lon":-111.8645,"city":"Centerville","region":"West Virginia","metro":"770","zip":"84014"},"connectiontype":2,"carrier":"CenturyLink","ifa":"ifa1","os":"other","ua":"Roku/DVP-9.30 (881.30E04400A)","ip":"68.2.243.20"},"bcat":["IAB26"],"user":{"id":"user1"},"app":{"id":"app1","storeurl":"https://channelstore.roku.com/en-ca/details/test1/fxnow","name":"fxnetworks.fxnow","bundle":"app1","cat":["IAB1"],"pagecat":["IAB1"],"publisher":{"id":"pub1"}}}`)

func main() {
	tagParams, err := populateTagParams(reqBody)
	if err == nil {
		fmt.Printf("Unable to parse %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("%+v\n", tagParams)
	ReleaseTagParams(tagParams)
}
