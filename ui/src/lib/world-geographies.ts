import { geoMercator, geoPath } from "d3-geo";
import { feature } from "topojson-client";
import type { FeatureCollection, GeoJsonProperties, Geometry } from "geojson";
import type { GeometryCollection, Topology } from "topojson-specification";

type WorldTopology = Topology<{
  countries: GeometryCollection<GeoJsonProperties>;
}>;

export interface WorldPath {
  key: string;
  id: string | number | undefined;
  name: string;
  path: string;
}

export function buildWorldPaths({
  topology,
  width,
  height,
  scale,
  center = [0, 0],
}: {
  topology: Record<string, unknown>;
  width: number;
  height: number;
  scale: number;
  center?: [number, number];
}): WorldPath[] {
  const countriesObject = (topology as Partial<WorldTopology>).objects?.countries;
  if (!countriesObject) return [];

  const collection = feature(
    topology as unknown as WorldTopology,
    countriesObject,
  ) as FeatureCollection<Geometry, GeoJsonProperties>;
  const projection = geoMercator()
    .scale(scale)
    .center(center)
    .translate([width / 2, height / 2]);
  const path = geoPath(projection);

  return collection.features.flatMap((geo, index) => {
    const d = path(geo);
    if (!d) return [];
    const id = geo.id;
    const name =
      typeof geo.properties?.name === "string"
        ? geo.properties.name
        : String(id ?? "Unknown");
    return [
      {
        key: String(id ?? index),
        id,
        name,
        path: d,
      },
    ];
  });
}
