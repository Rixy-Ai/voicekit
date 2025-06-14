// Copyright 2025 Rixy Ai.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metric

// ------------------------------------------------

type MetricConfig struct {
	Timestamper MetricTimestamperConfig `yaml:"timestamper_config,omitempty"`
	Collector   MetricsCollectorConfig  `yaml:"collector,omitempty"`
	Reporter    MetricsReporterConfig   `yaml:"reporter,omitempty"`
}

var (
	DefaultMetricConfig = MetricConfig{
		Timestamper: DefaultMetricTimestamperConfig,
		Collector:   DefaultMetricsCollectorConfig,
		Reporter:    DefaultMetricsReporterConfig,
	}
)
