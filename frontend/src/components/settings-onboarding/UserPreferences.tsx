import { PopoverHelp } from "@/components/shared/popover-help";
import { useViewDensity } from "@/components/shared/view-density-context";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import { DOWNLOAD_IMAGE_TYPE_OPTIONS } from "@/types/ui-options";

export function UserPreferencesCard() {
  // Download Defaults from User Preferences Store
  const downloadDefaultTypes = useUserPreferencesStore((state) => state.downloadDefaults);
  const setDownloadDefaultTypes = useUserPreferencesStore((state) => state.setDownloadDefaults);
  const showOnlyDownloadDefaults = useUserPreferencesStore((state) => state.showOnlyDownloadDefaults);
  const setShowOnlyDownloadDefaults = useUserPreferencesStore((state) => state.setShowOnlyDownloadDefaults);
  const showDateModified = useUserPreferencesStore((state) => state.showDateModified);
  const setShowDateModified = useUserPreferencesStore((state) => state.setShowDateModified);
  const enableSortByNewEpisode = useUserPreferencesStore((state) => state.enableSortByNewEpisode);
  const setEnableSortByNewEpisode = useUserPreferencesStore((state) => state.setEnableSortByNewEpisode);

  // View Density
  const { densityStep, setDensityStep } = useViewDensity();

  return (
    <>
      {/* View Density Preferences */}
      <Card className="p-5 mb-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold">View Density</h2>
          </div>
          <PopoverHelp ariaLabel="help-view-density">
            <p className="mb-2">
              Adjust the overall density of item listings across the application. This affects how many images are shown
              in the carousels.
            </p>
          </PopoverHelp>
        </div>
        <div className="mt-2">
          <span className="text-sm text-muted-foreground">
            Choose how compact or spacious you want item listings to appear:
          </span>
          <ul className="list-disc list-inside text-sm text-muted-foreground mt-1 mb-2">
            <li>
              <b>High Density:</b> More items fit on the screen with smaller images.
            </li>
            <li>
              <b>Medium Density:</b> Balanced view with moderate image sizes.
            </li>
            <li>
              <b>Low Density:</b> Larger images with more spacing, fewer items visible at once.
            </li>
          </ul>
          <div className="mt-2 flex items-center gap-2">
            <span className="text-xs">View Density:</span>
            <button
              className={cn("px-2 py-1 rounded", densityStep === 0 && "bg-primary text-white")}
              onClick={() => setDensityStep(0)}
            >
              High
            </button>
            <button
              className={cn("px-2 py-1 rounded", densityStep === 1 && "bg-primary text-white")}
              onClick={() => setDensityStep(1)}
            >
              Medium
            </button>
            <button
              className={cn("px-2 py-1 rounded", densityStep === 2 && "bg-primary text-white")}
              onClick={() => setDensityStep(2)}
            >
              Low
            </button>
          </div>
        </div>
      </Card>

      {/* Download Defaults Preferences */}
      <Card className="p-5">
        <div className="flex items-center justify-between">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold">Download Defaults</h2>
          </div>
          <PopoverHelp ariaLabel="help-default-image-types">
            <p className="mb-2">
              Select which image types you want auto-checked for each download. This will let you avoid unchecking them
              manually for each download.
            </p>
            <p className="text-muted-foreground">Click a badge to toggle it on or off.</p>
          </PopoverHelp>
        </div>
        <div className="flex flex-wrap gap-2">
          {DOWNLOAD_IMAGE_TYPE_OPTIONS.map((opt) => (
            <Badge
              key={opt.value}
              className={cn(
                "cursor-pointer text-sm px-3 py-1 font-normal transition",
                downloadDefaultTypes.includes(opt.value)
                  ? "bg-primary text-primary-foreground active:scale-95 hover:brightness-120"
                  : "bg-muted text-muted-foreground border hover:text-accent-foreground"
              )}
              variant={downloadDefaultTypes.includes(opt.value) ? "default" : "outline"}
              onClick={() => {
                if (downloadDefaultTypes.includes(opt.value)) {
                  // Only allow removal if more than one type is selected
                  if (downloadDefaultTypes.length > 1) {
                    setDownloadDefaultTypes(downloadDefaultTypes.filter((t) => t !== opt.value));
                  }
                } else {
                  setDownloadDefaultTypes([...downloadDefaultTypes, opt.value]);
                }
              }}
              style={
                downloadDefaultTypes.includes(opt.value) && downloadDefaultTypes.length === 1
                  ? { opacity: 0.5, pointerEvents: "none" }
                  : undefined
              }
            >
              {opt.label}s
            </Badge>
          ))}
        </div>
        <div className="flex items-center justify-between mt-3">
          <div className="flex items-center gap-5">
            <Label>Only Show Download Defaults</Label>
            <Switch
              checked={showOnlyDownloadDefaults}
              onCheckedChange={() => setShowOnlyDownloadDefaults(!showOnlyDownloadDefaults)}
            />
          </div>
          <PopoverHelp ariaLabel="help-filter-image-types">
            <p className="mb-2">
              If checked, only sets that contain at least one of the selected image types will be shown.
            </p>
            <p className="text-muted-foreground">
              This is global setting that will be applied to all media items and user sets. You can always change this
              setting here or in the Filters section of the Media Item Page. Section.
            </p>
          </PopoverHelp>
        </div>
      </Card>

      <Card className="p-5 mt-6">
        <div className="flex items-center justify-between">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold">Home Page Sorting</h2>
          </div>
          <PopoverHelp ariaLabel="help-sort-new-episode">
            <p className="mb-2">
              When enabled, the <b>New Episode Added</b> sort option will be available on the Home page.
            </p>
            <p className="text-muted-foreground">
              This option requires an extra request to your media server to fetch the latest episode dates for all
              shows. Disabling it can significantly speed up initial page loading for large libraries.
            </p>
          </PopoverHelp>
        </div>
        <div className="flex items-center gap-5 mt-3">
          <Label>Enable &quot;Sort by New Episode Added&quot;</Label>
          <Switch
            checked={enableSortByNewEpisode}
            onCheckedChange={() => setEnableSortByNewEpisode(!enableSortByNewEpisode)}
          />
        </div>
        {!enableSortByNewEpisode && (
          <p className="text-sm text-muted-foreground mt-2">
            The &quot;New Episode Added&quot; sort option is hidden on the Home page. Re-enable this to restore it.
          </p>
        )}
      </Card>
          <PopoverHelp ariaLabel="media-item-filter-date-modified">
            <p className="mb-2">When enabled, the "Date Modified" for each image will be shown under the image.</p>
            <p className="text-muted-foreground">
              This date is based on the last time the image was modified within MediUX.
            </p>
          </PopoverHelp>
        </div>
        <div className="flex items-center gap-5 mt-3">
          <Label>Show Date Modified</Label>
          <Switch checked={showDateModified} onCheckedChange={() => setShowDateModified(!showDateModified)} />
        </div>
      </Card>
    </>
  );
}
