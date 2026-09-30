import { describe, expect, it } from "vitest";
import { isRemoteAccessEnabled } from "../../utils/remoteAccess";

describe("isRemoteAccessEnabled", () => {
	it("is off when settings are missing or the field is absent", () => {
		expect(isRemoteAccessEnabled(undefined)).toBe(false);
		expect(isRemoteAccessEnabled(null)).toBe(false);
		expect(isRemoteAccessEnabled({})).toBe(false);
	});
	it("is off for anything but boolean true", () => {
		expect(isRemoteAccessEnabled({ remote_access_enabled: "true" })).toBe(
			false,
		);
		expect(isRemoteAccessEnabled({ remote_access_enabled: 1 })).toBe(false);
		expect(isRemoteAccessEnabled({ remote_access_enabled: false })).toBe(false);
	});
	it("is on only for true", () => {
		expect(isRemoteAccessEnabled({ remote_access_enabled: true })).toBe(true);
	});
});
