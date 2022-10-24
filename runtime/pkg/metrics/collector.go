// Copyright 2022 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/matrixorigin/matrixone-operator/runtime/pkg/reconciler"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"sync"
)

type MetricsCollector struct {
	cli    client.Client
	name   string
	logger logr.Logger

	resourceMap map[string]metricDesc
	sync.RWMutex
}

type metricDesc struct {
	objList client.ObjectList
	obj     client.Object
	desc    *prometheus.Desc
}

func NewMetricsCollector(name string, cli client.Client) *MetricsCollector {
	return &MetricsCollector{
		name:        name,
		cli:         cli,
		resourceMap: map[string]metricDesc{},
	}
}

func (c *MetricsCollector) RegisterResource(objList client.ObjectList) error {
	var name string
	t := reflect.TypeOf(objList)
	if t.Kind() == reflect.Ptr {
		name = t.Elem().Name()
	} else {
		name = t.Name()
	}
	name = strings.ToLower(strings.TrimSuffix(name, "List"))
	c.Lock()
	defer c.Unlock()
	c.resourceMap[name] = metricDesc{
		objList: objList,
		desc: prometheus.NewDesc(
			prometheus.BuildFQName(c.name, name, "condition"),
			fmt.Sprintf("current status condition of %s/%s", c.name, name),
			[]string{"namespace", "name", "type"},
			nil),
	}
	return nil
}

func (c *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.RLock()
	defer c.RUnlock()
	for _, d := range c.resourceMap {
		ch <- d.desc
	}
}

func (c *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	c.RLock()
	defer c.RUnlock()
	ctx := context.Background()
	for name, d := range c.resourceMap {
		objList := d.objList.DeepCopyObject().(client.ObjectList)
		if err := c.cli.List(ctx, objList); err != nil {
			c.logger.Error(err, "cannot list items", "kind", name)
		}
		items, err := meta.ExtractList(objList)
		if err != nil {
			c.logger.Error(err, "cannot extract items from List", "kind", name)
		}
		desc := d.desc
		for i := range items {
			if c, ok := items[i].(reconciler.Conditional); ok {
				m := items[i].(metav1.Object)
				for _, condition := range c.GetConditions() {
					v := 0.0
					if condition.Status == metav1.ConditionTrue {
						v = 1.0
					}
					ch <- prometheus.MustNewConstMetric(desc,
						prometheus.GaugeValue,
						v,
						m.GetNamespace(),
						m.GetName(),
						condition.Type,
					)
				}
			}
		}
	}
}
