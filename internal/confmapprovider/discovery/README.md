# Discovery confmap.Provider (Experimental)

**This component should not be considered stable. At this time its functionality is provided for testing and validation purposes only.**

The Discovery [confmap.Provider](https://pkg.go.dev/go.opentelemetry.io/collector/confmap#readme-provider) provides
the ability to define Collector service config through individual component yaml mappings in a `config.d` directory:

```mermaid
graph LR
  config.d[/config.d/] --> 1>service.yaml]
  subgraph 1a[service.yaml]
    1 --> 1a1[[pipelines:<br>metrics:<br>receivers:<br>- otlp<br>exporters:<br>- logging]]
  end
  config.d --> 2[/exporters/]
  subgraph 2a[exporters]
    2 --> 2a1>otlp.yaml]
    2a1 --> 2b1[[otlp:<br>endpoint: 1.2.3.4:2345]]
    2 --> 2a2>logging.yaml]
    2a2 --> 2b2[[logging:<br>verbosity: detailed]]
  end
  config.d --> 3[/extensions/]
  subgraph 3a[extensions]
    3 --> 3a1>zpages.yaml]
    3a1 --> 3b1[[zpages:<br>endpoint: 0.0.0.0:12345]]
    3 --> 3a2>health-check.yaml]
    3a2 --> 3b2[[health_check:<br>path: /health]]
  end
  config.d --> 4[/processors/]
  subgraph 4a[processors]
    4 --> 4a1>batch.yaml]
    4a1 --> 4b1[[batch:<br>]]
    4 --> 4a2>resource-detection.yaml]
    4a2 --> 4b2[[resourcedetection:<br>detectors:<br>- system]]
  end
  config.d --> 5[/receivers/]
  subgraph 5a[receivers]
    5 --> 5a1>otlp.yaml]
    5a1 --> 5b1[[otlp:<br>protocols:<br>grpc:]]
  end
```

This component is currently supported in the Collector settings via the following commandline options:

| option         | environment variable | default                        | description                                                                                                                             |
|----------------|----------------------|--------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------|
| `--configd`    | none                 | disabled                       | Whether to enable `config.d` functionality for final Collector config content.                                                          |
| `--config-dir` | `SPLUNK_CONFIG_DIR`  | `/etc/otel/collector/config.d` | The root `config.d` directory to walk for component directories and yaml mapping files.                                                 |
| `--dry-run`    | none                 | disabled                       | Whether to report the final assembled config contents to stdout before immediately exiting. This can be used with or without `config.d` |

To source only `config.d` content and not an additional or default configuration file, the `--config` option or
`SPLUNK_CONFIG` environment variable must be set to `/dev/null` or an arbitrary empty file:

```bash
$ # run the Collector without a config file using components from a local ./config.d config directory,
$ # printing the config to stdout before exiting instead of starting the Collector service:
$ bin/otelcol --config /dev/null --configd --config-dir ./config.d --dry-run
2023/02/24 19:54:23 settings.go:331: Set config to [/dev/null]
2023/02/24 19:54:23 settings.go:384: Set ballast to 168 MiB
2023/02/24 19:54:23 settings.go:400: Set memory limit to 460 MiB
exporters:
  logging:
    verbosity: detailed
  otlp:
    endpoint: 1.2.3.4:2345
extensions:
  health_check:
    path: /health
  zpages:
    endpoint: 0.0.0.0:1234
processors:
  batch: {}
  resourcedetection:
    detectors:
    - system
receivers:
  otlp:
    protocols:
      grpc: null
service:
  pipelines:
    metrics:
      exporters:
      - logging
      receivers:
      - otlp
```

## Discovery Mode

This component also provides a `--discovery [--dry-run]` option compatible with `config.d` that attempts to instantiate
any `.discovery.yaml` receivers using corresponding `.discovery.yaml` observers in a "preflight" Collector service.
Discovery mode will:

1. Load and attempt to start any observers in `config.d/extensions/<name>.discovery.yaml`.
1. Load and attempt to start any receiver blocks in `config.d/receivers/<name>.discovery.yaml` in a
[Discovery Receiver](../../receiver/discoveryreceiver/README.md) instance to receive discovery events from all
successfully started observers.
1. Wait 10s or the configured `SPLUNK_DISCOVERY_DURATION` environment variable [`time.Duration`](https://pkg.go.dev/time#ParseDuration).
1. Embed any receiver instances' configs resulting in a `discovery.status` of `successful` inside a `receiver_creator/discovery` receiver's configuration to be passed to the final Collector service config in a new or existing `service::pipelines::metrics::receivers` sequence (or outputted w/ `--dry-run`). Any required observers will be added to `service::extensions`.
1. Log any receiver resulting in a `discovery.status` of `partial` with the configured guidance for setting any relevant discovery properties.
1. Stop all temporary components before continuing on to the actual Collector service (or exiting early with `--dry-run`).


By default, the Discovery mode is provided with pre-made discovery config components in [`bundle.d`](./bundle/README.md).


### Discovery properties

Configuring discovery components is performed by merging discovery properties with the config.d receivers
and extensions `*.discovery.yaml` files. Discovery properties are of the form:

```yaml
splunk.discovery.receivers.<receiver-type(/name)>.config.<field>(<::subfield>)*: <value>
splunk.discovery.extensions.<observer-type(/name)>.config.<field>(<::subfield>)*: <value>
splunk.discovery.receivers.<receiver-type(/name)>.enabled: <true or false>
splunk.discovery.extensions.<observer-type(/name)>.enabled: <true or false>

# Examples
splunk.discovery.receivers.prometheus_simple.config.labels::my_label: my_label_value
splunk.discovery.receivers.prometheus_simple.enabled: true

splunk.discovery.extensions.docker_observer.config.endpoint: tcp://localhost:8080
splunk.discovery.extensions.k8s_observer.enabled: false
```

These properties can be in `config.d/properties.discovery.yaml` or specified at run time with `--set` command line options.

Each discovery property also has an equivalent environment variable form using `_x<hex pair>_` encoded delimiters for
non-word characters `[^a-zA-Z0-9_]`:

```bash
SPLUNK_DISCOVERY_RECEIVERS_receiver_x2d_type_x2f_receiver_x2d_name_CONFIG_field_x3a__x3a_subfield=value
SPLUNK_DISCOVERY_EXTENSIONS_observer_x2d_type_x2f_observer_x2d_name_CONFIG_field_x3a__x3a_subfield=value
SPLUNK_DISCOVERY_RECEIVERS_receiver_x2d_type_x2f_receiver_x2d_name_ENABLED=<true or false>
SPLUNK_DISCOVERY_EXTENSIONS_observer_x2d_type_x2f_observer_x2d_name_ENABLED=<true or false>

# Examples
SPLUNK_DISCOVERY_RECEIVERS_prometheus_simple_CONFIG_labels_x3a__x3a_my_label="my_username"
SPLUNK_DISCOVERY_RECEIVERS_prometheus_simple_ENABLED=true

SPLUNK_DISCOVERY_EXTENSIONS_docker_observer_CONFIG_endpoint="tcp://localhost:8080"
SPLUNK_DISCOVERY_EXTENSIONS_k8s_observer_ENABLED=false
```

The priority order for discovery config content from lowest to highest is:

1. `config.d/<receivers or extensions>/*.discovery.yaml` file content (lowest).
2. `config.d/properties.discovery.yaml` file content.
3. `SPLUNK_DISCOVERY_<xyz>` environment variables available to the collector process.
4. `--set splunk.discovery.<xyz>` commandline options (highest).
