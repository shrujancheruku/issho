import type { Crew } from '@issho/api-client';

import { withoutExamples } from '@/features/plans/format';
import type { Names } from '@/features/plans/names';

/**
 * A crew's name, "{name} crew", from what it's named after: the activity
 * (in both display languages) or the area (stored names only; area names
 * are never machine-translated).
 */
export function crewName(crew: Pick<Crew, 'activityId' | 'areaId'>, names: { activity: (id: string) => Names; area: (id: string) => Names }, template: (name: string) => string): Names {
  const n = crew.activityId ? names.activity(crew.activityId) : names.area(crew.areaId);
  return {
    name: template(withoutExamples(n.name)),
    secondary: n.secondary ? template(withoutExamples(n.secondary)) : '',
  };
}
