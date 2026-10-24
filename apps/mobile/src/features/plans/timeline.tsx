import type { PlanStep } from "@issho/api-client";
import { Linking, Pressable, StyleSheet, View } from "react-native";

import { GoogleMapsAttribution } from "@/components/google-maps-attribution";
import { ThemedText } from "@/components/themed-text";
import { Brand, Spacing } from "@/constants/theme";
import { useI18n } from "@/i18n/i18n-provider";

import { withoutExamples } from "./format";
import type { usePlanNames } from "./names";

/**
 * The plan's stops in order, as a timeline: a dot per stop on a vertical
 * line, with the walk between them on the line. Each stop is the activity,
 * then the venue (Google's name in both display languages, fetched live),
 * its address and a link to it in Google Maps. A walk's stops have no
 * activity: the place is the heading, followed by the time there and a
 * note on why it's worth the stop.
 */
/** `closed` leaves out what only matters for a plan still to happen. */
export function Timeline({
  steps,
  names,
  closed = false,
}: {
  steps: PlanStep[];
  names: ReturnType<typeof usePlanNames>;
  closed?: boolean;
}) {
  const { t, showing } = useI18n();
  // Google's terms: Places data shown without a Google map needs attribution.
  const fromGoogle = steps.some((s) => s.venueName || s.venueAddress);
  return (
    <View>
      {steps.map((step, i) => {
        const activity = names.activity(step.activityId);
        const last = i === steps.length - 1;
        // Venue names come primary first; follow the language switch like the rest.
        const [venue, venueOther] =
          showing === "secondary" && step.venueSecondaryName
            ? [step.venueSecondaryName, step.venueName]
            : [step.venueName, step.venueSecondaryName];
        const note =
          showing === "secondary" && step.secondaryNote
            ? step.secondaryNote
            : step.note;
        // What happens here; a walk's stop is just its place.
        const label = step.meeting
          ? t("plan.meetHere")
          : step.finish
            ? t("plan.finishHere")
            : step.activityId
              ? withoutExamples(activity.name)
              : "";
        const next = steps[i + 1];
        // A walk's time and note, shown even if its place couldn't be fetched.
        const details = (
          <>
            {step.minutes > 0 && (
              <ThemedText type="small" themeColor="textSecondary">
                {t("plan.stay", { minutes: step.minutes })}
              </ThemedText>
            )}
            {!!note && <ThemedText style={styles.note}>{note}</ThemedText>}
          </>
        );
        return (
          <View key={i} style={styles.stop}>
            <View style={styles.rail}>
              <View style={[styles.dot, { backgroundColor: Brand.gold }]} />
              {!last && (
                <View
                  style={[
                    styles.line,
                    // The thread between stops.
                    { backgroundColor: Brand.gold },
                  ]}
                />
              )}
            </View>
            <View style={[styles.body, !last && styles.bodyGap]}>
              {!!label && (
                <ThemedText type="smallBold" style={styles.activity}>
                  {label}
                </ThemedText>
              )}
              {venue ? (
                <>
                  <ThemedText
                    type={label ? "default" : "smallBold"}
                    style={!label && styles.activity}
                  >
                    {venue}
                    {!!venueOther && (
                      <ThemedText themeColor="textSecondary">
                        {" "}
                        · {venueOther}
                      </ThemedText>
                    )}
                  </ThemedText>
                  {!!step.venueAddress && (
                    <ThemedText type="small" themeColor="textSecondary">
                      {step.venueAddress}
                    </ThemedText>
                  )}
                  {details}
                  {!!step.venueMapsUrl && (
                    <Pressable
                      accessibilityRole="link"
                      onPress={() => void Linking.openURL(step.venueMapsUrl)}
                      hitSlop={6}
                    >
                      <ThemedText type="small" themeColor="link" style={styles.link}>
                        {t("plan.openInMaps")} ↗
                      </ThemedText>
                    </Pressable>
                  )}
                </>
              ) : note ? (
                details
              ) : (
                i === 0 &&
                !closed && (
                  <ThemedText type="small" themeColor="textSecondary">
                    {t("plan.meetingPointLater")}
                  </ThemedText>
                )
              )}
              {!!next?.walkMinutes && (
                <ThemedText
                  type="small"
                  themeColor="textSecondary"
                  style={styles.walk}
                >
                  {t("plan.walk", { minutes: next.walkMinutes })}
                </ThemedText>
              )}
            </View>
          </View>
        );
      })}
      {fromGoogle && <GoogleMapsAttribution />}
    </View>
  );
}

const DOT = 12;

const styles = StyleSheet.create({
  stop: { flexDirection: "row", gap: Spacing.three },
  // The rail is as tall as the stop: dot at the top, line filling the rest.
  rail: { width: DOT, alignItems: "center" },
  dot: { width: DOT, height: DOT, borderRadius: DOT / 2, marginTop: 4 },
  line: { width: 2, flex: 1, marginTop: Spacing.one },
  body: { flex: 1, gap: Spacing.half },
  bodyGap: { paddingBottom: Spacing.four },
  activity: { fontSize: 16, lineHeight: 22 },
  link: { marginTop: Spacing.half },
  note: { marginTop: Spacing.one },
  walk: { marginTop: Spacing.three, fontStyle: "italic" },
});
