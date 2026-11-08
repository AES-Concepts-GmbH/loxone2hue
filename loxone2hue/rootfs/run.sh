#!/usr/bin/with-contenv bashio
# Home Assistant Add-on Startup Script for Loxone2HUE Gateway

# Use persistent config path - survives restarts and updates
CONFIG_PATH=/data/options.json
PERSISTENT_CONFIG_PATH=/config/loxone2hue/config.yaml

# Create persistent config directory
mkdir -p /config/loxone2hue

# Read configuration from Home Assistant options
HUE_BRIDGE_IP=$(bashio::config 'hue_bridge_ip')
HUE_APPLICATION_KEY=$(bashio::config 'hue_application_key')
LOG_LEVEL=$(bashio::config 'log_level')

# If no persistent config exists, create default
if [ ! -f "${PERSISTENT_CONFIG_PATH}" ]; then
    bashio::log.info "Creating initial config at ${PERSISTENT_CONFIG_PATH}"
    cat > ${PERSISTENT_CONFIG_PATH} << EOF
server:
  port: 8080
  host: "0.0.0.0"

hue:
  bridge_ip: "${HUE_BRIDGE_IP}"
  application_key: "${HUE_APPLICATION_KEY}"

loxone:
  enabled: true

logging:
  level: "${LOG_LEVEL}"
  format: "json"

mappings: []
EOF
else
    bashio::log.info "Using existing config from ${PERSISTENT_CONFIG_PATH}"

    # Replace a single key inside one top-level YAML block.
    #
    # The previous version matched the bare key anywhere in the file, so an
    # unrelated line (a mapping description, a nested "level:") was rewritten
    # too. The address range confines the substitution to the block, and the
    # replacement is escaped so a value containing / & or \ cannot corrupt the
    # command. The | delimiter keeps URLs readable.
    set_yaml_key() {
        local block="$1" key="$2" value="$3"
        local escaped
        escaped=$(printf '%s' "${value}" | sed -e 's/[\\&|]/\\&/g')
        sed -i "/^${block}:/,/^[^[:space:]]/ s|^\( *${key}:\).*|\1 \"${escaped}\"|" \
            "${PERSISTENT_CONFIG_PATH}"
    }

    # Update log level from HA options if set
    if [ -n "${LOG_LEVEL}" ]; then
        set_yaml_key logging level "${LOG_LEVEL}"
    fi

    # Only override HUE credentials if explicitly set in HA options (not empty)
    if [ -n "${HUE_BRIDGE_IP}" ]; then
        bashio::log.info "Updating bridge_ip from HA options"
        set_yaml_key hue bridge_ip "${HUE_BRIDGE_IP}"
    fi
    if [ -n "${HUE_APPLICATION_KEY}" ]; then
        bashio::log.info "Updating application_key from HA options"
        set_yaml_key hue application_key "${HUE_APPLICATION_KEY}"
    fi
fi

bashio::log.info "Starting Loxone2HUE Gateway..."
bashio::log.info "Web UI available at: $(bashio::addon.ingress_url)"
bashio::log.info "Config path: ${PERSISTENT_CONFIG_PATH}"

# Change to app directory and start gateway with persistent config
cd /app
# Ingress gets a port of its own (ingress_port in config.yaml): requests the
# Supervisor proxies there are already signed in to Home Assistant and skip
# the gateway login. Port 8080 keeps the login for Loxone and direct access.
exec ./gateway -config ${PERSISTENT_CONFIG_PATH} -ingress-port 18099
