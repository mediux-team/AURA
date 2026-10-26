"use client";

import { makePlural } from "@/helper/make_plural";
import { ReturnErrorMessage } from "@/services/api-error-return";
import { ClearLogFiles } from "@/services/logs/clear";
import { DownloadDebugBundle } from "@/services/logs/debug-bundle";
import type { GetLogContents_Response } from "@/services/logs/get";
import { GetLogContents } from "@/services/logs/get";
import { Bug, EllipsisIcon, Loader2, SaveIcon, XCircle } from "lucide-react";
import { toast } from "sonner";

import { useEffect, useState } from "react";

import { CustomPagination } from "@/components/shared/custom-pagination";
import { ErrorMessage } from "@/components/shared/error-message";
import { FilterLogs } from "@/components/shared/filter-logs";
import Loader from "@/components/shared/loader";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Separator } from "@/components/ui/separator";
import { Small } from "@/components/ui/typography";

import { cn } from "@/lib/cn";
import { useLogsPageStore } from "@/lib/stores/page-store-logs";

import type { APIResponse, LogAction, LogData } from "@/types/api/api-response";

export default function LogsPage() {
  // States - Loading & Error
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<APIResponse<GetLogContents_Response> | null>(null);
  // States - Responses and Data
  const [logEntries, setLogEntries] = useState<LogData[]>([]);
  const [totalLogEntries, setTotalLogEntries] = useState<number>(0);
  const [possibleActionsPaths, setPossibleActionsPaths] = useState<Record<string, { label: string; section: string }>>(
    {}
  );

  // States - Filters & Pagination
  const {
    levelsFilter,
    setLevelsFilter,
    statusFilter,
    setStatusFilter,
    actionsFilter,
    setActionsFilter,
    currentPage,
    setCurrentPage,
    itemsPerPage,
    setItemsPerPage,
  } = useLogsPageStore();

  useEffect(() => {
    const fetchLogs = async () => {
      try {
        setLoading(true);
        const response = await GetLogContents(levelsFilter, statusFilter, actionsFilter, itemsPerPage, currentPage);
        if (response.status === "error") {
          setError(response);
          setLogEntries([]);
          setPossibleActionsPaths({});
          setTotalLogEntries(0);
          return;
        }
        const logEntries = response.data?.log_entries || [];

        setLogEntries(logEntries);
        setPossibleActionsPaths(response.data?.possible_actions_paths || {});
        setTotalLogEntries(response.data?.total_log_entries || 0);
        setError(null);
      } catch (error) {
        setError({
          status: "error",
          error: {
            message: "Failed to fetch logs",
            function: "fetchLogs",
            line_number: 0,
            help: "Check server connectivity and permissions.",
            detail: error instanceof Error ? { message: error.message } : {},
          },
        });
        setLogEntries([]);
        setPossibleActionsPaths({});
        setTotalLogEntries(0);
      } finally {
        setLoading(false);
      }
    };

    fetchLogs();
  }, [actionsFilter, currentPage, itemsPerPage, levelsFilter, statusFilter]);

  const clearLogsFromToday = async () => {
    try {
      const response = await ClearLogFiles(true);
      if (response.status === "error") {
        toast.error(response.error?.message || "Failed to clear old logs");
        return;
      }
      toast.success(response.data?.message || "Logs from today cleared successfully");
      setLogEntries([]);
      setTotalLogEntries(0);
      setCurrentPage(1);
    } catch {
      toast.error("An unexpected error occurred");
    }
  };

  const [creatingBundle, setCreatingBundle] = useState(false);
  const downloadDebugBundle = async () => {
    setCreatingBundle(true);
    const result = await DownloadDebugBundle();
    setCreatingBundle(false);
    if (!result.ok) {
      toast.error(`Failed to download the debug bundle: ${result.error}`);
      return;
    }
    toast.success("Debug bundle downloaded");
  };

  // Calculate total pages
  const totalPages = Math.ceil(totalLogEntries / itemsPerPage);

  return (
    <div className="container mx-auto px-2 py-4 min-h-screen flex flex-col items-center">
      <Card className="w-full">
        <CardHeader>
          <div className="flex flex-col md:flex-row md:items-center md:justify-between w-full space-y-2 md:space-y-0">
            <h2 className="text-lg font-medium text-center md:text-left whitespace-nowrap">Application Logs</h2>
            <div className={cn("flex flex-row flex-wrap justify-center md:justify-end gap-2 px-2 w-full max-w-full")}>
              <FilterLogs
                levelsFilter={levelsFilter}
                setLevelsFilter={setLevelsFilter}
                statusFilter={statusFilter}
                setStatusFilter={setStatusFilter}
                actionsOptions={possibleActionsPaths}
                actionsFilter={actionsFilter}
                setActionsFilter={setActionsFilter}
                setCurrentPage={setCurrentPage}
                itemsPerPage={itemsPerPage}
                setItemsPerPage={setItemsPerPage}
              />

              <Button variant="outline" onClick={downloadDebugBundle} disabled={creatingBundle}>
                {creatingBundle ? <Loader2 className="h-4 w-4 animate-spin" /> : <Bug className="h-4 w-4" />}
                Debug Bundle
              </Button>

              <Button variant="destructive" onClick={clearLogsFromToday}>
                Clear Today's Logs
              </Button>
            </div>
          </div>
        </CardHeader>

        <CardContent>
          {loading ? (
            <Loader />
          ) : error ? (
            <ErrorMessage error={error} />
          ) : (!logEntries || logEntries.length === 0) && !error && !loading ? (
            <div className="w-full">
              <ErrorMessage
                error={ReturnErrorMessage<string>(
                  [
                    `No Log Entries found`,
                    levelsFilter.length > 0
                      ? `with ${makePlural(levelsFilter, "level")} ${levelsFilter.map((lvl) => `"${lvl}"`).join(", ")}`
                      : null,
                    statusFilter.length > 0
                      ? `with ${makePlural(statusFilter, "status")} ${statusFilter.map((st) => `"${st}"`).join(", ")}`
                      : null,
                    actionsFilter.length > 0
                      ? `with ${makePlural(actionsFilter, "action")} ${actionsFilter
                          .map((act) => {
                            const label = possibleActionsPaths[act]?.label;
                            return `"${label || act}"`;
                          })
                          .join(", ")}`
                      : null,
                  ]
                    .filter(Boolean)
                    .join("\n")
                )}
              />
              <div className="text-center text-muted-foreground mt-4">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setLevelsFilter([]);
                    setStatusFilter([]);
                    setActionsFilter([]);
                    setCurrentPage(1);
                  }}
                  className="text-sm"
                >
                  <XCircle className="inline mr-1" />
                  Clear All Filters
                </Button>
              </div>
            </div>
          ) : (
            <>
              <LogList logEntries={logEntries} possibleActionsPaths={possibleActionsPaths} />

              {/* Pagination */}
              {itemsPerPage && (
                <CustomPagination
                  currentPage={currentPage}
                  totalPages={totalPages}
                  setCurrentPage={setCurrentPage}
                  scrollToTop={true}
                  filterItemsLength={totalLogEntries}
                  itemsPerPage={itemsPerPage}
                />
              )}
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function routeActionKey(method?: string, path?: string): string {
  const m = (method || "").trim().toUpperCase();
  const p = (path || "").trim();
  if (!m || !p) return p;
  return `${m}:${p}`;
}

function LogList({
  logEntries,
  possibleActionsPaths,
}: {
  logEntries: LogData[];
  possibleActionsPaths: Record<string, { label: string; section: string }>;
}) {
  return (
    <div className="space-y-1">
      {logEntries.map((log, idx) => {
        let mainLabel = "";
        if (log.route?.path) {
          const key = log.route.action || routeActionKey(log.route.method, log.route.path);
          const actionPath = possibleActionsPaths[key] || possibleActionsPaths[log.route.path]; // fallback
          mainLabel = actionPath?.label || key || log.route.path;
        } else if (log.actions && log.actions.length > 0) {
          mainLabel = log.message || log.actions[0].name || "Background Task";
        }

        return (
          <Card
            key={idx}
            className={cn(
              "w-full",
              "border-2 border-transparent transition-colors",
              log.level === "error" ? "border-red-500" : ""
            )}
          >
            <CardHeader className={cn("gap-0 mb-0")}>
              <div className="flex flex-col">
                <div className="flex flex-row items-center justify-between mb-1 w-full">
                  {mainLabel && <Small className="font-medium text-lg flex-1">{mainLabel}</Small>}
                  <>
                    {/* Drop Down Menu Options To Export Log Line */}
                    <DropdownMenu>
                      <DropdownMenuTrigger
                        asChild
                        className={cn("cursor-pointer ml-2", "hover:brightness-120 active:scale-95 transition-all")}
                      >
                        <EllipsisIcon className="h-5 w-5 text-muted-foreground" />
                      </DropdownMenuTrigger>
                      <DropdownMenuContent className="w-56 md:w-64" side="bottom" align="end">
                        <DropdownMenuItem
                          className="cursor-pointer flex items-center active:scale-95 hover:brightness-120"
                          onClick={() => {
                            const logDataStr = JSON.stringify(log, null, 2);
                            const blob = new Blob([logDataStr], {
                              type: "application/json",
                            });
                            const url = URL.createObjectURL(blob);
                            const link = document.createElement("a");
                            link.href = url;
                            const downloadName = mainLabel ? mainLabel.replace(/\s+/g, "_").toLowerCase() : "";
                            link.download = `log-${downloadName}-${log.status}.json`;
                            document.body.appendChild(link);
                            link.click();
                            document.body.removeChild(link);
                            URL.revokeObjectURL(url);
                          }}
                        >
                          <SaveIcon className="w-6 h-6 mr-2" />
                          Export {log.status.toUpperCase()} Log
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </>
                </div>
                {log.elapsed_us !== undefined && (
                  <Small className="text-sm text-muted-foreground whitespace-nowrap">
                    Elapsed Time: {formatElapsedMicroseconds(log.elapsed_us)}
                  </Small>
                )}
                {log.route && (
                  <Accordion type="single" collapsible>
                    <AccordionItem value="route-info">
                      <AccordionTrigger
                        className={cn(
                          "cursor-pointer",
                          "hover:underline-none focus:underline-none underline-none hover:no-underline focus:no-underline"
                        )}
                      >
                        <span className="text-sm font-semibold text-muted-foreground">Route Info</span>
                      </AccordionTrigger>
                      <AccordionContent>
                        <div className="flex flex-col">
                          {log.route.method && (
                            <Small className="text-sm text-muted-foreground">Method: {log.route.method}</Small>
                          )}
                          {log.route.path && (
                            <Small className="text-sm text-muted-foreground">Path: {log.route.path}</Small>
                          )}
                          {log.route.ip && <Small className="text-sm text-muted-foreground">IP: {log.route.ip}</Small>}
                          {log.route.response_bytes !== undefined && (
                            <Small className="text-sm text-muted-foreground">
                              Response Size: {formatBytes(log.route.response_bytes)}
                            </Small>
                          )}
                          {log.route.params && Object.keys(log.route.params).length > 0 && (
                            <Small className="text-sm text-muted-foreground">
                              Params:{" "}
                              {Object.entries(log.route.params).map(([key, value]) => (
                                <span key={key} className="mr-2">
                                  <strong>{key}:</strong> {value}
                                </span>
                              ))}
                            </Small>
                          )}
                        </div>
                      </AccordionContent>
                    </AccordionItem>
                  </Accordion>
                )}
              </div>
            </CardHeader>
            <CardContent className="mt-0">
              {Array.isArray(log.actions) && log.actions.length > 0 && (
                <Accordion type="multiple">
                  {log.actions.map((action: LogAction, i: number) => [
                    <LogActionAccordion key={action.name + action.timestamp + i} action={action} />,
                    <Separator key={"sep-" + action.name + action.timestamp + i} className="my-2" />,
                  ])}
                </Accordion>
              )}
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}

function LogActionAccordion({ action }: { action: LogAction }) {
  const hasExpandableContent =
    (action.sub_actions && action.sub_actions.length > 0) ||
    action.result !== undefined ||
    action.warnings !== undefined;

  return (
    <AccordionItem value={action.name + (action.timestamp || "")}>
      {hasExpandableContent ? (
        <AccordionTrigger
          className={cn(
            "flex flex-col items-start gap-1",
            { "cursor-pointer": action.sub_actions && action.sub_actions.length > 0 },
            "hover:underline-none focus:underline-none underline-none hover:no-underline focus:no-underline"
          )}
        >
          <LogActionHeader action={action} />
        </AccordionTrigger>
      ) : (
        <div
          className={cn(
            "flex flex-col items-start gap-1 align-top my-2",
            { "cursor-pointer": action.sub_actions && action.sub_actions.length > 0 },
            "hover:underline-none focus:underline-none underline-none hover:no-underline focus:no-underline"
          )}
        >
          <LogActionHeader action={action} />
        </div>
      )}

      {hasExpandableContent && (
        <AccordionContent>
          {action.result && (
            <div className="text-xs mt-1">
              Results:
              <pre className="rounded p-2 mt-1 text-xs overflow-x-auto max-h-40 whitespace-pre break-words w-full">
                {JSON.stringify(action.result, null, 2)}
              </pre>
            </div>
          )}
          {action.warnings && (
            <div className="text-xs text-yellow-600 mt-1">
              Warnings:
              <pre className="rounded p-2 mt-1 text-xs overflow-x-auto max-h-40 whitespace-pre break-words w-full">
                {JSON.stringify(action.warnings, null, 2)}
              </pre>
            </div>
          )}
          {action.sub_actions && action.sub_actions.length > 0 && (
            <AccordionContent>
              <Accordion type="multiple" className="border-muted">
                {action.sub_actions.map((sub: LogAction, idx: number) => (
                  <LogActionAccordion key={idx} action={sub} />
                ))}
              </Accordion>
            </AccordionContent>
          )}
        </AccordionContent>
      )}
    </AccordionItem>
  );
}

function LogActionHeader({ action }: { action: LogAction }) {
  return (
    <>
      <div className="font-medium text-sm">{action.name}</div>
      <div className="flex flex-row gap-2 text-xs text-muted-foreground flex-wrap">
        {action.level && (
          <span
            className={
              action.level === "error"
                ? "text-red-500"
                : action.level === "warn"
                  ? "text-yellow-600"
                  : action.level === "info"
                    ? "text-blue-600"
                    : action.level === "debug"
                      ? "text-green-600"
                      : "text-purple-600"
            }
          >
            {action.level?.toUpperCase()}
          </span>
        )}
        {action.level && action.status && " • "}
        {action.status && (
          <span
            className={
              action.status === "error"
                ? "text-red-500"
                : action.status === "warn"
                  ? "text-yellow-600"
                  : "text-green-600"
            }
          >
            {action.status.toUpperCase()}
          </span>
        )}
      </div>
      <div className="text-xs text-muted-foreground">
        {action.timestamp && <>{formatFullTimestamp(action.timestamp)}</>}
        {action.timestamp && action.elapsed_us !== undefined && " • "}
        {action.elapsed_us !== undefined && <>{formatElapsedMicroseconds(action.elapsed_us)}</>}
      </div>
      {action.error && <div className="text-xs text-red-500 mt-1">Error: {action.error.message}</div>}
    </>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 Bytes";
  const k = 1024;
  const sizes = ["Bytes", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

function formatElapsedMicroseconds(us: number): string {
  if (us < 1000) return `${us} µs`;
  if (us < 1_000_000) return `${(us / 1000).toFixed(2)} ms`;
  return `${(us / 1_000_000).toFixed(2)} s`;
}

function formatFullTimestamp(dateString: string): string {
  const d = new Date(dateString);
  const pad = (n: number, z = 2) => String(n).padStart(z, "0");
  const hours = d.getHours();
  const am_pm = hours >= 12 ? "PM" : "AM";
  const hour12 = hours % 12 === 0 ? 12 : hours % 12;
  return `${pad(d.getMonth() + 1)}/${pad(d.getDate())}/${d.getFullYear()} ${pad(hour12)}:${pad(d.getMinutes())}:${pad(d.getSeconds())} ${am_pm}`;
}
