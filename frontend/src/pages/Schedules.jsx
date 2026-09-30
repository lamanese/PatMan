import { Power, Wrench } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import { useAuth } from "../contexts/AuthContext";
import PatchSchedules from "./PatchSchedules";
import RebootSchedules from "./RebootSchedules";

// Patch and reboot schedules share one page. Each tab keeps the gate of its
// former standalone route (and of its server API), so a role only sees the
// schedules it may manage.
const Schedules = () => {
	const [searchParams, setSearchParams] = useSearchParams();
	const { canManagePatching, hasModule, hasPermission } = useAuth();

	const tabs = [];
	if (canManagePatching() && hasModule("patching")) {
		tabs.push({
			id: "patch",
			label: "Patch",
			icon: Wrench,
			Content: PatchSchedules,
		});
	}
	if (hasPermission("can_reboot_hosts")) {
		tabs.push({
			id: "reboot",
			label: "Reboot",
			icon: Power,
			Content: RebootSchedules,
		});
	}

	const requested = searchParams.get("tab");
	const active = tabs.find((t) => t.id === requested) ?? tabs[0];

	return (
		<div className="space-y-6">
			<div>
				<h1 className="text-2xl font-semibold text-secondary-900 dark:text-white">
					Schedules
				</h1>
				<p className="text-sm text-secondary-600 dark:text-white mt-1">
					Scheduled patch runs and reboots per host group
				</p>
			</div>

			{!active ? (
				<p className="text-sm text-secondary-600 dark:text-white">
					You don't have access to any schedules.
				</p>
			) : (
				<>
					<div className="border-b border-secondary-200 dark:border-secondary-600 overflow-x-auto scrollbar-hide">
						<nav
							className="-mb-px flex space-x-4 sm:space-x-8 px-4"
							aria-label="Tabs"
						>
							{tabs.map((tab) => {
								const Icon = tab.icon;
								return (
									<button
										key={tab.id}
										type="button"
										onClick={() => setSearchParams({ tab: tab.id })}
										className={`${
											active.id === tab.id
												? "border-primary-500 text-primary-600 dark:text-primary-400"
												: "border-transparent text-secondary-500 hover:text-secondary-700 hover:border-secondary-300 dark:text-white dark:hover:text-primary-400"
										} whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm flex items-center gap-2`}
									>
										<Icon className="h-4 w-4" />
										<span>{tab.label}</span>
									</button>
								);
							})}
						</nav>
					</div>

					<active.Content key={active.id} />
				</>
			)}
		</div>
	);
};

export default Schedules;
