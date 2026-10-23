import { FavoriteCreatorsCard } from "@/components/settings-onboarding/FavoriteCreators";
import { PopoverHelp } from "@/components/shared/popover-help";
import { useViewDensity } from "@/components/shared/view-density-context";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import { DOWNLOAD_DEFAULT_TYPE_OPTIONS } from "@/types/ui-options";

export function UserPreferencesCard() {
  // Download Defaults from User Preferences Store
  const downloadDefaultTypes = useUserPreferencesStore((state) => state.downloadDefaults);
  const setDownloadDefaultTypes = useUserPreferencesStore((state) => state.setDownloadDefaults);
  const showOnlyDownloadDefaults = useUserPreferencesStore((state) => state.showOnlyDownloadDefaults);
  const setShowOnlyDownloadDefaults = useUserPreferencesStore((state) => state.setShowOnlyDownloadDefaults);
  const showDateModified = useUserPreferencesStore((state) => state.showDateModified);
  const setShowDateModified = useUserPreferencesStore((state) => state.setShowDateModified);
  const defaultAddToQueue = useUserPreferencesStore((state) => state.defaultAddToQueue);
  const setDefaultAddToQueue = useUserPreferencesStore((state) => state.setDefaultAddToQueue);
  const defaultAddToDBOnly = useUserPreferencesStore((state) => state.defaultAddToDBOnly);
  const setDefaultAddToDBOnly = useUserPreferencesStore((state) => state.setDefaultAddToDBOnly);
  const defaultAutoAddNewCollectionItems = useUserPreferencesStore((state) => state.defaultAutoAddNewCollectionItems);
  const setDefaultAutoAddNewCollectionItems = useUserPreferencesStore(
    (state) => state.setDefaultAutoAddNewCollectionItems
  );

  // View Density
  const { densityStep, setDensityStep } = useViewDensity();

  return (
    <div className="space-y-4">
      {/* View Density Preferences */}
      <Card className="gap-4 border-muted p-4 sm:p-5">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold text-blue-500">View Density</h2>
          <PopoverHelp ariaLabel="help-view-density">
            <p className="mb-2">
              Adjust the overall density of item listings across the application. This affects how many images are shown
              in the carousels.
            </p>
          </PopoverHelp>
        </div>
        <div>
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
      <Card className="gap-4 border-muted p-4 sm:p-5">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold text-blue-500">Download Defaults</h2>
          <PopoverHelp ariaLabel="help-default-image-types">
            <p className="mb-2">
              Select which image types you want auto-checked for each download. This will let you avoid unchecking them
              manually for each download.
            </p>
            <p className="text-muted-foreground">Click a badge to toggle it on or off.</p>
          </PopoverHelp>
        </div>
        <div className="flex flex-wrap gap-2">
          {DOWNLOAD_DEFAULT_TYPE_OPTIONS.map((opt) => (
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
        <div className="flex items-center justify-between gap-4">
          <Label>Only Show Download Defaults</Label>
          <div className="flex items-center gap-2">
            <Switch
              checked={showOnlyDownloadDefaults}
              onCheckedChange={() => setShowOnlyDownloadDefaults(!showOnlyDownloadDefaults)}
            />
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
        </div>
        <div className="flex items-center justify-between gap-4">
          <Label>Add to Queue by Default</Label>
          <div className="flex items-center gap-2">
            <Switch checked={defaultAddToQueue} onCheckedChange={() => setDefaultAddToQueue(!defaultAddToQueue)} />
            <PopoverHelp ariaLabel="help-default-add-to-queue">
              <p className="mb-2">If checked, 'Add to Queue' will be selected when opening the download modal.</p>
              <p className="text-muted-foreground">
                You can still uncheck it in the download modal to download straight away.
              </p>
            </PopoverHelp>
          </div>
        </div>
        <div className="flex items-center justify-between gap-4">
          <Label>Add to Database Only by Default</Label>
          <div className="flex items-center gap-2">
            <Switch checked={defaultAddToDBOnly} onCheckedChange={() => setDefaultAddToDBOnly(!defaultAddToDBOnly)} />
            <PopoverHelp ariaLabel="help-default-add-to-db-only">
              <p className="mb-2">
                If checked, 'Add to Database Only' will be selected for each item when opening the download modal, so
                the sets are saved without downloading any images.
              </p>
              <p className="text-muted-foreground">
                You can still uncheck it in the download modal to download the images.
              </p>
            </PopoverHelp>
          </div>
        </div>
        <div className="flex items-center justify-between gap-4">
          <Label>Auto Add New Collection Items by Default</Label>
          <div className="flex items-center gap-2">
            <Switch
              checked={defaultAutoAddNewCollectionItems}
              onCheckedChange={() => setDefaultAutoAddNewCollectionItems(!defaultAutoAddNewCollectionItems)}
            />
            <PopoverHelp ariaLabel="help-default-auto-add-new-collection-items">
              <p className="mb-2">
                If checked, 'Auto Add New Collection Items' will be selected when opening the download modal for a
                collection set. Movies added to the collection later then get their images from the set automatically.
              </p>
              <p className="text-muted-foreground">You can still uncheck it in the download modal.</p>
            </PopoverHelp>
          </div>
        </div>
      </Card>

      {/* MediUX Images Preferences */}
      <Card className="gap-4 border-muted p-4 sm:p-5">
        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold text-blue-500">MediUX Images</h2>
        </div>
        <div className="flex items-center justify-between gap-4">
          <Label>Show Date Modified</Label>
          <div className="flex items-center gap-2">
            <Switch checked={showDateModified} onCheckedChange={() => setShowDateModified(!showDateModified)} />
            <PopoverHelp ariaLabel="media-item-filter-date-modified">
              <p className="mb-2">When enabled, the "Date Modified" for each image will be shown under the image.</p>
              <p className="text-muted-foreground">
                This date is based on the last time the image was modified within MediUX.
              </p>
            </PopoverHelp>
          </div>
        </div>
      </Card>

      {/* Favorite Creators */}
      <FavoriteCreatorsCard />
    </div>
  );
}
