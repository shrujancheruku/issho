import { getOption, hasOption, type DescMessage } from "@bufbuild/protobuf";
import { field as fieldRules } from "./gen/buf/validate/validate_pb";

/**
 * The longest a string field may be, in characters, read from the `max_len`
 * rule on the field in the .proto (`[(buf.validate.field).string.max_len = N]`).
 * The server enforces that same rule, so inputs use this for `maxLength`
 * instead of a copied constant. An installed build keeps the limit it was
 * built with; the server stays the authority.
 *
 * `field` is the proto name (`display_name`) or the TypeScript one
 * (`displayName`). Throws when the field has no such rule, so a rule dropped
 * from the proto fails loudly instead of silently removing a limit.
 */
export function maxLen(schema: DescMessage, field: string): number {
  const f = schema.fields.find((x) => x.name === field || x.localName === field);
  if (!f) throw new Error(`${schema.typeName} has no field ${field}`);
  if (hasOption(f, fieldRules)) {
    const type = getOption(f, fieldRules).type;
    if (type.case === "string" && type.value.maxLen !== undefined) return Number(type.value.maxLen);
  }
  throw new Error(`${schema.typeName}.${f.name} has no string max_len rule`);
}
