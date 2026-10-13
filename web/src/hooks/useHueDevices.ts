import { useState, useEffect, useCallback } from 'react';
import { Light, Group, Scene, Sensor, WSMessage } from '../types';
import * as api from '../services/api';
import { useWebSocket } from './useWebSocket';

export function useHueDevices() {
  const [lights, setLights] = useState<Light[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [scenes, setScenes] = useState<Scene[]>([]);
  const [sensors, setSensors] = useState<Sensor[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const handleMessage = useCallback((message: WSMessage) => {
    if (message.type === 'status') {
      setLights((prev) =>
        prev.map((light) =>
          light.id === message.device
            ? { ...light, state: { ...light.state, ...message.state } }
            : light
        )
      );
    } else if (message.type === 'sensor_update') {
      setSensors((prev) =>
        prev.map((s) =>
          s.id === message.sensor.id ? message.sensor : s
        )
      );
    } else if (message.type === 'group_update') {
      // Aggregated room/zone state from the bridge's grouped_light service.
      setGroups((prev) =>
        prev.map((g) =>
          g.id === message.group.id ? { ...g, state: message.group.state } : g
        )
      );
    }
  }, []);

  const { isConnected } = useWebSocket({ onMessage: handleMessage });

  const fetchData = useCallback(async (showLoading = true) => {
    try {
      if (showLoading) setLoading(true);
      setError(null);

      const [devicesRes, groupsRes, scenesRes, sensorsRes] = await Promise.all([
        api.getDevices(),
        api.getGroups(),
        api.getScenes(),
        api.getSensors(),
      ]);

      setLights(devicesRes.devices || []);
      setGroups(groupsRes.groups || []);
      setScenes(scenesRes.scenes || []);
      setSensors(sensorsRes.sensors || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch devices');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const setLightState = useCallback(
    async (id: string, on?: boolean, brightness?: number) => {
      try {
        await api.setDevice(id, { on, brightness });
        // Optimistic update
        setLights((prev) =>
          prev.map((light) =>
            light.id === id
              ? {
                  ...light,
                  state: {
                    ...light.state,
                    ...(on !== undefined && { on }),
                    ...(brightness !== undefined && { brightness }),
                  },
                }
              : light
          )
        );
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to update light');
      }
    },
    []
  );

  const setGroupState = useCallback(
    async (id: string, on?: boolean, brightness?: number) => {
      try {
        // Optimistic update for group state
        setGroups((prev) =>
          prev.map((group) =>
            group.id === id
              ? {
                  ...group,
                  state: {
                    ...group.state,
                    ...(on !== undefined && { any_on: on, all_on: on }),
                  },
                }
              : group
          )
        );

        await api.setGroup(id, { on, brightness });

        // Refresh data after a short delay to get actual state. Without
        // `false` the loading spinner replaced the lists and every card lost
        // its local state (open UDP section, slider position).
        setTimeout(() => fetchData(false), 1000);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to update group');
        // Revert on error
        fetchData(false);
      }
    },
    [fetchData]
  );

  const activateScene = useCallback(async (id: string) => {
    try {
      await api.activateScene(id);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to activate scene');
    }
  }, []);

  const refresh = useCallback(() => fetchData(false), [fetchData]);

  return {
    lights,
    groups,
    scenes,
    sensors,
    loading,
    error,
    isConnected,
    refresh,
    setLightState,
    setGroupState,
    activateScene,
  };
}
