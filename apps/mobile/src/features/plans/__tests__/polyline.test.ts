/// <reference types="jest" />
import { decodePolyline } from '../polyline';

it("decodes Google's documented example", () => {
  // From Google's polyline algorithm page.
  expect(decodePolyline('_p~iF~ps|U_ulLnnqC_mqNvxq`@')).toEqual([
    { lat: 38.5, lng: -120.2 },
    { lat: 40.7, lng: -120.95 },
    { lat: 43.252, lng: -126.453 },
  ]);
  expect(decodePolyline('')).toEqual([]);
  expect(() => decodePolyline('_p~iF~ps|U_')).toThrow('truncated');
});
