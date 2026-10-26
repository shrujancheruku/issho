import { useRef } from 'react';
import { StyleSheet } from 'react-native';
import type MapViewType from 'react-native-maps';

import { useTheme } from '@/hooks/use-theme';

import { dropDelay } from './drop-in';
import { bounds, initialRegion, type Pin } from './map-data';
import { BadgeMarker, MapUnavailable, maps } from './native-maps';
import { pinColors } from './pin-color';

export type PlansMapProps = {
  pins: Pin[];
  selected: string | null;
  onSelect: (areaId: string) => void;
  /** Where to start when there are no pins: the person's home area. */
  home?: { lat: number; lng: number };
  /** How much of the map's bottom is covered (by a sheet): pins are framed above it. */
  bottomInset?: number;
};

/**
 * The map of plans on phones: Google Maps (react-native-maps with the Google
 * provider, as Google's Places policy needs for venue data on a map), one
 * pin per area, coloured by its best plan, with how many plans it has.
 * The pins drop in one after another the first time the map opens.
 * The web version is plans-map.web.tsx.
 */
export function PlansMap(props: PlansMapProps) {
  if (!maps) return <MapUnavailable />;
  return <NativeMap {...props} />;
}

function NativeMap({ pins, selected, onSelect, home, bottomInset = 0 }: PlansMapProps) {
  const { default: MapView, PROVIDER_GOOGLE } = maps!;
  const theme = useTheme();
  const map = useRef<MapViewType>(null);
  const box = bounds(pins);
  // Start framed on the pins (or the home area). Without an initial region
  // the map opens on the whole world, and on Android it ignores
  // fitToCoordinates until it has loaded, so framing has to happen here and
  // again in onMapReady, not in an effect right after mounting.
  const region = initialRegion(box, home) ?? undefined;
  const fit = () => {
    if (!box) return;
    map.current?.fitToCoordinates(
      [
        { latitude: box.north, longitude: box.west },
        { latitude: box.south, longitude: box.east },
      ],
      { edgePadding: { top: 40, right: 40, bottom: 40 + bottomInset, left: 40 }, animated: false },
    );
  };

  return (
    <MapView
      ref={map}
      provider={PROVIDER_GOOGLE}
      style={StyleSheet.absoluteFill}
      initialRegion={region}
      onMapReady={fit}
      toolbarEnabled={false}
      showsPointsOfInterests={false}>
      {pins.map((p, i) => (
        <BadgeMarker
          key={p.areaId}
          lat={p.lat}
          lng={p.lng}
          label={String(p.plans.length)}
          colors={pinColors(p.level, theme)}
          selected={p.areaId === selected}
          appear={{ id: `area:${p.areaId}`, delay: dropDelay(i) }}
          onPress={() => onSelect(p.areaId)}
        />
      ))}
    </MapView>
  );
}
