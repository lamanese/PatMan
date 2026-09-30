// Remote access (browser SSH terminal, RDP) is off unless the server was
// started with PM_ENABLE_REMOTE_ACCESS=true. Read fail-closed: a missing
// field (403 on the settings query, old server) means off.
export function isRemoteAccessEnabled(settings) {
	return settings?.remote_access_enabled === true;
}
