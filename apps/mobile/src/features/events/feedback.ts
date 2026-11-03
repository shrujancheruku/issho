import type { FeedbackAnswer } from '@/features/events/data';

/** The two lists of mates a person can put someone in after an outing. */
export type MateList = 'picks' | 'avoid';

/**
 * Toggles a mate in one list. Someone is in at most one of the two lists
 * (you can't both want to meet them again and rather not), so choosing them
 * in one clears the other. Pure; returns a new answer.
 */
export function toggleMate(answer: FeedbackAnswer, list: MateList, mate: string): FeedbackAnswer {
  const other = list === 'picks' ? 'avoid' : 'picks';
  const on = answer[list].includes(mate);
  return {
    ...answer,
    [list]: on ? answer[list].filter((p) => p !== mate) : [...answer[list], mate],
    [other]: answer[other].filter((p) => p !== mate),
  };
}

/** Someone who didn't go has no ratings or picks to send, whatever was tapped before. */
export function submittedAnswer(answer: FeedbackAnswer): FeedbackAnswer {
  return answer.attended ? answer : { attended: false, planRating: 0, groupRating: 0, picks: [], avoid: [] };
}
