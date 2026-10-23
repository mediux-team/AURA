"use client";

import { Check, Filter, Settings } from "lucide-react";

import { useState } from "react";

import Link from "next/link";

import { PopoverHelp } from "@/components/shared/popover-help";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { ToggleGroup } from "@/components/ui/toggle-group";

import { cn } from "@/lib/cn";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import { DOWNLOAD_DEFAULT_TYPE_OPTIONS, TYPE_DOWNLOAD_DEFAULT_TYPE_OPTIONS } from "@/types/ui-options";

type MediaItemFilterProps = {
  numberOfActiveFilters?: number;
  hiddenCount: number;
  showHiddenUsers: boolean;
  handleShowHiddenUsers: (val: boolean) => void;
  hasTitleCards: boolean;
  showOnlyTitlecardSets: boolean;
  handleShowSetsWithTitleCardsOnly: (val: boolean) => void;
  showOnlyDownloadDefaults: boolean;
  filterByLanguage: string[];
  setFilterByLanguage: (languages: string[]) => void;
};

function MediaItemFilterContent({
  hiddenCount,
  showHiddenUsers,
  handleShowHiddenUsers,
  hasTitleCards,
  showOnlyTitlecardSets,
  handleShowSetsWithTitleCardsOnly,
  filterByLanguage,
  setFilterByLanguage,
}: MediaItemFilterProps) {
  const downloadDefaultsTypes = useUserPreferencesStore((state) => state.downloadDefaults);
  const setDownloadDefaultsTypes = useUserPreferencesStore((state) => state.setDownloadDefaults);
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

  return (
    <div className="flex-grow space-y-4 overflow-y-auto px-4 py-2">
      <div className="flex flex-col">
        {/* Download Defaults */}
        <div className="flex items-center space-x-2 justify-between">
          <Label className="text-md font-semibold block">Download Defaults</Label>

          <PopoverHelp ariaLabel="help-default-image-types">
            <p className="mb-2">
              Select which image types you want auto-checked for each download. This will let you avoid unchecking them
              manually for each download.
            </p>
            <p className="text-muted-foreground">Click a badge to toggle it on or off.</p>
          </PopoverHelp>
        </div>
        <ToggleGroup
          type="multiple"
          className="flex flex-wrap gap-2 ml-2 mt-2"
          value={downloadDefaultsTypes}
          onValueChange={(value: TYPE_DOWNLOAD_DEFAULT_TYPE_OPTIONS[]) => {
            // Ensure at least one type is always selected
            if (value.length === 0) return;
            setDownloadDefaultsTypes(value);
          }}
        >
          {DOWNLOAD_DEFAULT_TYPE_OPTIONS.map((opt) => (
            <Badge
              key={opt.value}
              className={cn(
                "cursor-pointer text-sm px-3 py-1 font-normal transition active:scale-95",
                downloadDefaultsTypes.includes(opt.value)
                  ? "bg-primary text-primary-foreground hover:brightness-120"
                  : "bg-muted text-muted-foreground border hover:text-accent-foreground"
              )}
              variant={downloadDefaultsTypes.includes(opt.value) ? "default" : "outline"}
              onClick={() => {
                if (downloadDefaultsTypes.includes(opt.value)) {
                  // Only allow removal if more than one type is selected
                  if (downloadDefaultsTypes.length > 1) {
                    setDownloadDefaultsTypes(downloadDefaultsTypes.filter((t) => t !== opt.value));
                  }
                } else {
                  setDownloadDefaultsTypes([...downloadDefaultsTypes, opt.value]);
                }
              }}
              style={
                downloadDefaultsTypes.includes(opt.value) && downloadDefaultsTypes.length === 1
                  ? { opacity: 0.5, pointerEvents: "none" }
                  : undefined
              }
            >
              {opt.label}s
            </Badge>
          ))}
        </ToggleGroup>
        <div className="flex items-center space-x-2 justify-between mt-4">
          <div className="flex items-center space-x-2">
            <Switch
              className="ml-0"
              checked={showOnlyDownloadDefaults}
              onCheckedChange={() => setShowOnlyDownloadDefaults(!showOnlyDownloadDefaults)}
            />{" "}
            <Label>Only show selected image types</Label>
          </div>

          <PopoverHelp ariaLabel="help-filter-image-types">
            <p className="mb-2">
              If checked, only sets that contain at least one of the selected image types will be shown.
            </p>
            <p className="text-muted-foreground">
              This is a global setting that will be applied to all media items and user sets. You can always change this
              setting in this filter, or in the Settings Page{" "}
              <Link href="/settings#preferences-section" className="underline">
                User Preferences
              </Link>{" "}
              Section.
            </p>
          </PopoverHelp>
        </div>
        <div className="flex items-center space-x-2 justify-between mt-4">
          <div className="flex items-center space-x-2">
            <Switch
              className="ml-0"
              checked={defaultAddToQueue}
              onCheckedChange={() => setDefaultAddToQueue(!defaultAddToQueue)}
            />{" "}
            <Label>Add to Queue by default</Label>
          </div>

          <PopoverHelp ariaLabel="help-default-add-to-queue">
            <p className="mb-2">If checked, 'Add to Queue' will be selected when opening the download modal.</p>
            <p className="text-muted-foreground">
              This is global setting that will be applied to all downloads. You can always change this setting in this
              filter, or in the Settings Page{" "}
              <Link href="/settings#preferences-section" className="underline">
                User Preferences
              </Link>{" "}
              Section.
            </p>
          </PopoverHelp>
        </div>
        <div className="flex items-center space-x-2 justify-between mt-4">
          <div className="flex items-center space-x-2">
            <Switch
              className="ml-0"
              checked={defaultAddToDBOnly}
              onCheckedChange={() => setDefaultAddToDBOnly(!defaultAddToDBOnly)}
            />{" "}
            <Label>Add to Database Only by default</Label>
          </div>

          <PopoverHelp ariaLabel="help-default-add-to-db-only">
            <p className="mb-2">
              If checked, 'Add to Database Only' will be selected for each item when opening the download modal, so the
              sets are saved without downloading any images.
            </p>
            <p className="text-muted-foreground">
              You can always change this setting in this filter, or in the Settings Page{" "}
              <Link href="/settings#preferences-section" className="underline">
                User Preferences
              </Link>{" "}
              Section.
            </p>
          </PopoverHelp>
        </div>
        <div className="flex items-center space-x-2 justify-between mt-4">
          <div className="flex items-center space-x-2">
            <Switch
              className="ml-0"
              checked={defaultAutoAddNewCollectionItems}
              onCheckedChange={() => setDefaultAutoAddNewCollectionItems(!defaultAutoAddNewCollectionItems)}
            />{" "}
            <Label>Auto Add New Collection Items by default</Label>
          </div>

          <PopoverHelp ariaLabel="help-default-auto-add-new-collection-items">
            <p className="mb-2">
              If checked, 'Auto Add New Collection Items' will be selected when opening the download modal for a
              collection set. Movies added to the collection later then get their images from the set automatically.
            </p>
            <p className="text-muted-foreground">
              You can always change this setting in this filter, or in the Settings Page{" "}
              <Link href="/settings#preferences-section" className="underline">
                User Preferences
              </Link>{" "}
              Section.
            </p>
          </PopoverHelp>
        </div>

        {/* Hidden Users*/}
        {hiddenCount > 0 && (
          <>
            <Separator className="my-4 w-full" />
            <Label className="text-md font-semibold mb-1 block">Hidden Users</Label>
            <div className="justify-between flex items-center">
              <div className="flex items-center space-x-2">
                <Switch
                  className="ml-0"
                  checked={showHiddenUsers}
                  onCheckedChange={handleShowHiddenUsers}
                  disabled={hiddenCount === 0}
                />{" "}
                <Label>Show hidden users</Label>
              </div>
              <PopoverHelp ariaLabel="media-item-filter-hidden-users">
                <p className="mb-2">When enabled, sets from users you have hidden will be shown in the list.</p>
                <p className="text-muted-foreground">You can hide users directly in the MediUX site.</p>
              </PopoverHelp>
            </div>
          </>
        )}

        {/* Mandatory Titlecard Sets */}
        {hasTitleCards && (!showOnlyDownloadDefaults || downloadDefaultsTypes.includes("titlecard")) && (
          <>
            <Separator className="my-4 w-full" />
            <Label className="text-md font-semibold mb-1 block">Titlecard Filter</Label>
            <div className="justify-between flex items-center">
              <div className="flex items-center space-x-2">
                <Switch
                  className="ml-0"
                  checked={showOnlyTitlecardSets}
                  onCheckedChange={handleShowSetsWithTitleCardsOnly}
                />
                <Label>Only show sets with titlecards</Label>
              </div>
              <PopoverHelp ariaLabel="media-item-filter-titlecards">
                <p className="mb-2">When enabled, only sets that contain titlecards will be shown in the list.</p>
              </PopoverHelp>
            </div>
          </>
        )}
        <Separator className="my-4 w-full" />

        {/* Show Date Modified */}
        <div className="flex items-center justify-between">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold">MediUX Images</h2>
          </div>
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

        {/* Language Options */}
        <Separator className="my-4 w-full" />
        <Label className="text-md font-semibold mb-2 block">Languages</Label>
        {(() => {
          // Add the blank value for "All Languages" at the top
          const allLanguagesLabel = "All Languages";
          const sortedLanguages = [
            "", // blank value for "All Languages"
            ...languageOptions.filter((lang) => lang !== ""), // rest of the languages
          ].sort((a, b) => {
            // Always keep "" (All Languages) at the top
            if (a === "") return -1;
            if (b === "") return 1;
            const aSelected = filterByLanguage.includes(a);
            const bSelected = filterByLanguage.includes(b);
            if (aSelected === bSelected) return a.localeCompare(b);
            return aSelected ? -1 : 1;
          });

          return (
            <div className="flex flex-col gap-1 max-h-48 overflow-y-auto border p-2 rounded-md">
              {sortedLanguages.map((language) => (
                <div key={language || "__all__"} className="flex items-center space-x-2">
                  <Checkbox
                    id={`language-${language || "all"}`}
                    checked={
                      language === ""
                        ? filterByLanguage.length === 1 && filterByLanguage[0] === ""
                        : filterByLanguage.includes(language)
                    }
                    onCheckedChange={(checked) => {
                      if (language === "") {
                        // If "All Languages" is checked, clear all and set blank value
                        if (checked) setFilterByLanguage([""]);
                      } else {
                        // Remove "All Languages" if any specific language is checked
                        let newSelection = filterByLanguage.filter((l) => l !== "");
                        if (checked) {
                          newSelection = [...newSelection, language];
                        } else {
                          newSelection = newSelection.filter((l) => l !== language);
                        }
                        // If nothing is selected, default back to "All Languages"
                        if (newSelection.length === 0) {
                          setFilterByLanguage([""]);
                        } else {
                          setFilterByLanguage(newSelection);
                        }
                      }
                    }}
                  />
                  <Label htmlFor={`language-${language || "all"}`} className="flex items-center space-x-1">
                    <span>{language === "" ? allLanguagesLabel : language}</span>
                    {language !== "" && filterByLanguage.includes(language) && (
                      <Check className="h-4 w-4 text-green-500" />
                    )}
                  </Label>
                </div>
              ))}
            </div>
          );
        })()}
      </div>
    </div>
  );
}

export function MediaItemFilter({
  numberOfActiveFilters = 0,
  hiddenCount,
  showHiddenUsers,
  handleShowHiddenUsers,
  hasTitleCards,
  showOnlyTitlecardSets,
  handleShowSetsWithTitleCardsOnly,
  showOnlyDownloadDefaults,
  filterByLanguage,
  setFilterByLanguage,
}: MediaItemFilterProps) {
  // State - Open/Close Modal
  const [modalOpen, setModalOpen] = useState(false);

  return (
    <Dialog open={modalOpen} onOpenChange={setModalOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" className={cn(numberOfActiveFilters > 0 && "ring-1 ring-primary ring-offset-1")}>
          <Settings className="h-5 w-5" />
          Preferences & Filters {numberOfActiveFilters > 0 && `(${numberOfActiveFilters})`}
          <Filter className="h-5 w-5" />
        </Button>
      </DialogTrigger>
      <DialogContent
        className={cn("z-50", "max-h-[80vh] overflow-y-auto", "sm:max-w-[700px]", "border border-primary")}
      >
        <DialogHeader>
          <DialogTitle>Preferences & Filters</DialogTitle>
          <DialogDescription>
            Use the options below to choose your preferences and filter the poster sets.
          </DialogDescription>
        </DialogHeader>
        <Separator className="my-1 w-full" />
        <MediaItemFilterContent
          hiddenCount={hiddenCount}
          showHiddenUsers={showHiddenUsers}
          handleShowHiddenUsers={handleShowHiddenUsers}
          hasTitleCards={hasTitleCards}
          showOnlyTitlecardSets={showOnlyTitlecardSets}
          handleShowSetsWithTitleCardsOnly={handleShowSetsWithTitleCardsOnly}
          showOnlyDownloadDefaults={showOnlyDownloadDefaults}
          filterByLanguage={filterByLanguage}
          setFilterByLanguage={setFilterByLanguage}
        />
      </DialogContent>
    </Dialog>
  );
}

export const languageOptions = [
  "English",
  "Afar",
  "Abkhazian",
  "Avestan",
  "Afrikaans",
  "Akan",
  "Amharic",
  "Aragonese",
  "Arabic",
  "Assamese",
  "Avaric",
  "Aymara",
  "Azerbaijani",
  "Bashkir",
  "Belarusian",
  "Bulgarian",
  "Bislama",
  "Bambara",
  "Bengali",
  "Tibetan",
  "Breton",
  "Bosnian",
  "Catalan",
  "Chechen",
  "Chamorro",
  "Cantonese",
  "Corsican",
  "Cree",
  "Czech",
  "Old Church Slavonic",
  "Chuvash",
  "Welsh",
  "Danish",
  "German",
  "Divehi",
  "Dzongkha",
  "Ewe",
  "Greek",
  "Esperanto",
  "Spanish",
  "Estonian",
  "Basque",
  "Persian",
  "Fulah",
  "Finnish",
  "Fijian",
  "Faroese",
  "French",
  "Western Frisian",
  "Irish",
  "Scottish Gaelic",
  "Galician",
  "Guarani",
  "Gujarati",
  "Manx",
  "Hausa",
  "Hebrew",
  "Hindi",
  "Hiri Motu",
  "Croatian",
  "Haitian",
  "Hungarian",
  "Armenian",
  "Herero",
  "Interlingua",
  "Indonesian",
  "Interlingue",
  "Igbo",
  "Sichuan Yi",
  "Inupiaq",
  "Ido",
  "Icelandic",
  "Italian",
  "Inuktitut",
  "Japanese",
  "Javanese",
  "Georgian",
  "Kongo",
  "Kikuyu",
  "Kwanyama",
  "Kazakh",
  "Kalaallisut",
  "Khmer",
  "Kannada",
  "Korean",
  "Kanuri",
  "Kashmiri",
  "Kurdish",
  "Komi",
  "Cornish",
  "Kirghiz",
  "Latin",
  "Luxembourgish",
  "Ganda",
  "Limburgish",
  "Lingala",
  "Lao",
  "Lithuanian",
  "Luba-Katanga",
  "Latvian",
  "Malagasy",
  "Marshallese",
  "Maori",
  "Macedonian",
  "Malayalam",
  "Mongolian",
  "Moldavian",
  "Marathi",
  "Malay",
  "Maltese",
  "Burmese",
  "Nauru",
  "Norwegian Bokmål",
  "North Ndebele",
  "Nepali",
  "Ndonga",
  "Dutch",
  "Norwegian Nynorsk",
  "Norwegian",
  "South Ndebele",
  "Navajo",
  "Chichewa",
  "Occitan",
  "Ojibwa",
  "Oromo",
  "Oriya",
  "Ossetian",
  "Panjabi",
  "Pali",
  "Polish",
  "Pashto",
  "Portuguese",
  "Quechua",
  "Romansh",
  "Rundi",
  "Romanian",
  "Russian",
  "Kinyarwanda",
  "Sanskrit",
  "Sardinian",
  "Sindhi",
  "Northern Sami",
  "Sango",
  "Serbo-Croatian",
  "Sinhala",
  "Slovak",
  "Slovenian",
  "Samoan",
  "Shona",
  "Somali",
  "Albanian",
  "Serbian",
  "Swati",
  "Southern Sotho",
  "Sundanese",
  "Swedish",
  "Swahili",
  "Tamil",
  "Telugu",
  "Tajik",
  "Thai",
  "Tigrinya",
  "Turkmen",
  "Tagalog",
  "Tswana",
  "Tonga",
  "Turkish",
  "Tsonga",
  "Tatar",
  "Twi",
  "Tahitian",
  "Uighur",
  "Ukrainian",
  "Urdu",
  "Uzbek",
  "Venda",
  "Vietnamese",
  "Volapük",
  "Walloon",
  "Wolof",
  "Xhosa",
  "No Language / Textless",
  "Yiddish",
  "Yoruba",
  "Zhuang",
  "Chinese",
  "Zulu",
];
