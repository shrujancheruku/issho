import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { errorFromJson } from "@connectrpc/connect/protocol-connect";
import { describe, expect, it } from "vitest";
import { errorReason } from "./errors";
import { ErrorReason, ErrorReasonDetailSchema } from "./gen/issho/v1/errors_pb";

const errorFromJsonBody = (body: Parameters<typeof errorFromJson>[0]) => errorFromJson(body, undefined, new ConnectError("failed"));

describe("errorReason", () => {
  it("reads the reason detail of a Connect error", () => {
    const err = new ConnectError("account suspended", Code.PermissionDenied, undefined, [
      { desc: ErrorReasonDetailSchema, value: create(ErrorReasonDetailSchema, { reason: ErrorReason.SUSPENDED }) },
    ]);
    expect(errorReason(err)).toBe(ErrorReason.SUSPENDED);
  });

  it("reads a reason that crossed the wire as a Connect JSON error", () => {
    // The body connect-go sends: details carry the type and base64 proto bytes (08 04 = reason 4).
    const err = errorFromJsonBody({
      code: "failed_precondition",
      message: "too late to change your answer",
      details: [{ type: "issho.v1.ErrorReasonDetail", value: "CAQ" }],
    });
    expect(errorReason(err)).toBe(ErrorReason.TOO_LATE);
  });

  it("is undefined without the detail, and for errors that aren't Connect errors", () => {
    expect(errorReason(new ConnectError("nope", Code.NotFound))).toBeUndefined();
    expect(errorReason(new Error("network down"))).toBeUndefined();
    expect(errorReason(undefined)).toBeUndefined();
  });
});
