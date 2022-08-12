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

package dnset

const (
	dataVolume    = "data"
	dataPath      = "/var/lib/dnservice"
	configVolume  = "config"
	configPath    = "/etc/dnservice"
	PodNameEnvKey = "POD_NAME"
	servicePort   = 41010
	ListenIP      = "0.0.0.0"
	PodIPEnvKey   = "POD_IP"
	ServiceTypeDN = "DN"
	ConfigFile    = "dnservice.toml"
	Entrypoint    = "start.sh"
)
