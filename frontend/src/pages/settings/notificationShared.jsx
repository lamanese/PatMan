import { Bell, Globe, Mail } from "lucide-react";
import { SiDiscord, SiNtfy, SiSlack } from "react-icons/si";

/* Shared between the notification settings (destinations, routes, reports). */

export const CHANNEL_TYPES = [
	{
		value: "webhook",
		label: "Webhook",
		description: "Generic, Discord, or Slack",
		icon: Globe,
		brandIcons: { discord: SiDiscord, slack: SiSlack },
	},
	{
		value: "email",
		label: "Email",
		description: "SMTP delivery",
		icon: Mail,
	},
	{
		value: "ntfy",
		label: "ntfy",
		description: "Push notifications via ntfy.sh",
		icon: SiNtfy,
	},
	{
		value: "internal",
		label: "Internal Alerts",
		description: "Alert records in the Alerts tab",
		icon: Bell,
	},
];

export const channelIcon = (type) => {
	const ct = CHANNEL_TYPES.find((c) => c.value === type);
	if (!ct) return null;
	const Icon = ct.icon;
	return <Icon className="h-4 w-4" />;
};

export const INPUT =
	"w-full px-3 py-2 bg-white dark:bg-secondary-900 border border-secondary-300 dark:border-secondary-600 rounded-md text-sm text-secondary-900 dark:text-white focus:ring-2 focus:ring-primary-500 focus:border-primary-500 placeholder-secondary-400";
export const SELECT = `${INPUT} appearance-none`;
