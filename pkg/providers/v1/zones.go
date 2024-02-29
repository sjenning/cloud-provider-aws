/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"k8s.io/klog/v2"
)

type zoneDetails struct {
	name     string
	id       string
	zoneType string
}

type zoneCache struct {
	cloud             *Cloud
	mutex             sync.Mutex
	zoneNameToDetails map[string]zoneDetails
}

// Get the zone details by zone names and load from the cache if available as
// zone information should never change.
func (z *zoneCache) getZoneDetailsByNames(zoneNames []string) (map[string]zoneDetails, error) {
	if len(zoneNames) == 0 {
		return map[string]zoneDetails{}, nil
	}

	z.mutex.Lock()
	defer z.mutex.Unlock()

	if z.shouldPopulateCache(zoneNames) {
		// Populate the cache if it hasn't been populated yet
		err := z.populate()
		if err != nil {
			return nil, err
		}
	}

	requestedZoneDetails := map[string]zoneDetails{}
	for _, zone := range zoneNames {
		if zoneDetails, ok := z.zoneNameToDetails[zone]; ok {
			requestedZoneDetails[zone] = zoneDetails
		} else {
			klog.Warningf("Could not find zone %s", zone)
		}
	}

	return requestedZoneDetails, nil
}

// NOTE: This method is not thread safe and should not be called outside of getZoneDetailsByNames
func (z *zoneCache) shouldPopulateCache(zoneNames []string) bool {
	if len(z.zoneNameToDetails) == 0 {
		// Populate the cache if it hasn't been populated yet
		return true
	}

	// Make sure that we know about all of the AZs we're looking for.
	for _, zone := range zoneNames {
		if _, ok := z.zoneNameToDetails[zone]; !ok {
			klog.Infof("AZ %s not found in zone cache.", zone)
			return true
		}
	}

	return false
}

// Populates the zone cache. If cache is already populated, it will overwrite entries,
// which is useful when accounts get access to new zones.
// NOTE: This method is not thread safe and should not be called outside of getZoneDetailsByNames
func (z *zoneCache) populate() error {
	azRequest := &ec2.DescribeAvailabilityZonesInput{}
	zones, err := z.cloud.ec2.DescribeAvailabilityZones(azRequest)
	if err != nil {
		return fmt.Errorf("error describe availability zones: %q", err)
	}

	// Initialize the map if it's unset
	if len(z.zoneNameToDetails) == 0 {
		z.zoneNameToDetails = map[string]zoneDetails{}
	}

	for _, zone := range zones {
		name := aws.StringValue(zone.ZoneName)
		z.zoneNameToDetails[name] = zoneDetails{
			name:     name,
			id:       aws.StringValue(zone.ZoneId),
			zoneType: aws.StringValue(zone.ZoneType),
		}
	}

	return nil
}
