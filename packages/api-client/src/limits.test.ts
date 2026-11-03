import { describe, expect, it } from "vitest";
import { ChatRoomSchema, SendMessageRequestSchema } from "./gen/issho/v1/chat_pb";
import { LeaveCrewRequestSchema } from "./gen/issho/v1/crews_pb";
import { ReportPersonRequestSchema } from "./gen/issho/v1/events_pb";
import { ProfileSchema } from "./gen/issho/v1/profile_pb";
import { maxLen } from "./limits";

describe("maxLen", () => {
  it("returns the annotated limit of the chat body", () => {
    expect(maxLen(SendMessageRequestSchema, "body")).toBe(1000);
  });

  it("reads the other annotated fields, by proto or TypeScript name", () => {
    expect(maxLen(ReportPersonRequestSchema, "details")).toBe(1000);
    expect(maxLen(LeaveCrewRequestSchema, "details")).toBe(1000);
    expect(maxLen(ProfileSchema, "display_name")).toBe(40);
    expect(maxLen(ProfileSchema, "displayName")).toBe(40);
  });

  it("throws for a field that is missing or has no rule", () => {
    expect(() => maxLen(SendMessageRequestSchema, "nope")).toThrow(/no field/);
    expect(() => maxLen(ChatRoomSchema, "event_id")).toThrow(/no string max_len/);
  });
});
