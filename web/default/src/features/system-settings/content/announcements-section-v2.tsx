/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type MouseEvent,
} from "react";
import * as z from "zod";
import { useForm, type Path } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import {
  Edit,
  Image as ImageIcon,
  Plus,
  Trash2,
  Upload,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { toast } from "sonner";
import dayjs from "@/lib/dayjs";
import { officialWebsiteUrl } from "@/lib/origins";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { DateTimePicker } from "@/components/datetime-picker";
import { Dialog } from "@/components/dialog";
import { StatusBadge } from "@/components/status-badge";
import { SettingsSwitchField } from "../components/settings-form-layout";
import { SettingsSection } from "../components/settings-section";
import { useUpdateOption } from "../hooks/use-update-option";
import {
  ANNOUNCEMENT_LOGO_MAX_BYTES,
  BUILT_IN_MODEL_LOGOS,
  WEBSITE_LOCALES,
  WEBSITE_LOCALE_LABELS,
  announcementToFormValues,
  emptyLocaleTextMap,
  isSupportedAnnouncementLogo,
  normalizeAnnouncement,
  serializeAnnouncement,
  type Announcement,
  type AnnouncementType,
  type WebsiteLocale,
} from "./announcement-config";

type AnnouncementsSectionProps = {
  enabled: boolean;
  data: string;
};

const ANNOUNCEMENT_FORM_ID = "announcement-form";

const typeOptions: Array<{
  value: AnnouncementType;
  label: string;
  color: string;
  badgeVariant: "neutral" | "info" | "success" | "warning" | "danger";
}> = [
  {
    value: "default",
    label: "Default",
    color: "bg-gray-500",
    badgeVariant: "neutral",
  },
  {
    value: "ongoing",
    label: "Ongoing",
    color: "bg-blue-500",
    badgeVariant: "info",
  },
  {
    value: "success",
    label: "Success",
    color: "bg-green-500",
    badgeVariant: "success",
  },
  {
    value: "warning",
    label: "Warning",
    color: "bg-orange-500",
    badgeVariant: "warning",
  },
  {
    value: "error",
    label: "Error",
    color: "bg-red-500",
    badgeVariant: "danger",
  },
];

function createAnnouncementSchema(t: (key: string) => string) {
  const localeMap = z.record(z.string(), z.string());
  return z
    .object({
      content_i18n: localeMap,
      intro_i18n: localeMap,
      link_label_i18n: localeMap,
      publishDate: z.string().min(1, t("Publish date is required")),
      type: z.enum(["default", "ongoing", "success", "warning", "error"]),
      link: z
        .string()
        .max(500, t("Link URL must be less than 500 characters"))
        .optional(),
      logo: z.string().max(100_000, t("Logo data is too large")).optional(),
    })
    .superRefine((values, context) => {
      if (!Object.values(values.content_i18n).some((value) => value.trim())) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["content_i18n"],
          message: t("At least one language must have announcement content"),
        });
      }
      for (const [locale, value] of Object.entries(values.content_i18n)) {
        if (value.length > 500) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ["content_i18n", locale],
            message: t("Content must be less than 500 characters"),
          });
        }
      }
      for (const [locale, value] of Object.entries(values.intro_i18n)) {
        if (value.length > 200) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ["intro_i18n", locale],
            message: t("Intro must be less than 200 characters"),
          });
        }
      }
      for (const [locale, value] of Object.entries(values.link_label_i18n)) {
        if (value.length > 100) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ["link_label_i18n", locale],
            message: t("Link text must be less than 100 characters"),
          });
        }
      }
      const logo = values.logo?.trim();
      if (logo) {
        if (logo.length > ANNOUNCEMENT_LOGO_MAX_BYTES) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ["logo"],
            message: t("Logo file must be 64 KB or smaller"),
          });
        } else if (!isSupportedAnnouncementLogo(logo)) {
          context.addIssue({
            code: z.ZodIssueCode.custom,
            path: ["logo"],
            message: t("Logo must be a supported image file"),
          });
        }
      }
    });
}

type AnnouncementFormSchema = ReturnType<typeof createAnnouncementSchema>;
type AnnouncementFormValuesFromSchema = z.infer<AnnouncementFormSchema>;

function localizedPath(
  field: "content_i18n" | "intro_i18n" | "link_label_i18n",
  locale: WebsiteLocale,
): Path<AnnouncementFormValuesFromSchema> {
  return `${field}.${locale}` as Path<AnnouncementFormValuesFromSchema>;
}

export function AnnouncementsSectionV2({
  enabled,
  data,
}: AnnouncementsSectionProps) {
  const { t, i18n } = useTranslation();
  const updateOption = useUpdateOption();
  const [announcements, setAnnouncements] = useState<Announcement[]>([]);
  const [isEnabled, setIsEnabled] = useState(enabled);
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [showDialog, setShowDialog] = useState(false);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [editingAnnouncement, setEditingAnnouncement] =
    useState<Announcement | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<"single" | "batch">(
    "single",
  );
  const [activeLocale, setActiveLocale] = useState<WebsiteLocale>("en");
  const logoInputRef = useRef<HTMLInputElement>(null);

  const schema = useMemo(() => createAnnouncementSchema(t), [t]);
  const form = useForm<AnnouncementFormValuesFromSchema>({
    resolver: zodResolver(schema),
    defaultValues: {
      content_i18n: emptyLocaleTextMap(),
      intro_i18n: emptyLocaleTextMap(),
      link_label_i18n: emptyLocaleTextMap(),
      publishDate: new Date().toISOString(),
      type: "default",
      link: "",
      logo: "",
    },
  });

  useEffect(() => {
    try {
      const parsed: unknown = JSON.parse(data || "[]");
      setAnnouncements(
        Array.isArray(parsed)
          ? parsed
              .map((item, index) => normalizeAnnouncement(item, index + 1))
              .filter((item): item is Announcement => item !== null)
          : [],
      );
    } catch {
      setAnnouncements([]);
    }
  }, [data]);

  useEffect(() => setIsEnabled(enabled), [enabled]);

  useEffect(() => {
    const current = i18n.resolvedLanguage?.split("-")[0] as WebsiteLocale;
    if (WEBSITE_LOCALES.includes(current)) {
      setActiveLocale(current);
    }
  }, [i18n.resolvedLanguage]);

  const persistAnnouncements = async (next: Announcement[]) => {
    const response = await updateOption.mutateAsync({
      key: "console_setting.announcements",
      value: JSON.stringify(next),
    });
    if (!response.success) return false;
    setAnnouncements(next);
    return true;
  };

  const handleToggleEnabled = async (checked: boolean) => {
    const response = await updateOption.mutateAsync({
      key: "console_setting.announcements_enabled",
      value: checked,
    });
    if (response.success) setIsEnabled(checked);
  };

  const handleAdd = () => {
    setEditingAnnouncement(null);
    form.reset({
      content_i18n: emptyLocaleTextMap(),
      intro_i18n: emptyLocaleTextMap(),
      link_label_i18n: emptyLocaleTextMap(),
      publishDate: new Date().toISOString(),
      type: "default",
      link: "",
      logo: "",
    });
    setActiveLocale("en");
    setShowDialog(true);
  };

  const handleEdit = (announcement: Announcement) => {
    setEditingAnnouncement(announcement);
    form.reset(announcementToFormValues(announcement));
    setActiveLocale("en");
    setShowDialog(true);
  };

  const handleDelete = (announcement: Announcement) => {
    setEditingAnnouncement(announcement);
    setDeleteTarget("single");
    setShowDeleteDialog(true);
  };

  const handleBatchDelete = () => {
    if (selectedIds.length === 0) {
      toast.error(t("Please select items to delete"));
      return;
    }
    setDeleteTarget("batch");
    setShowDeleteDialog(true);
  };

  const confirmDelete = async (event?: MouseEvent<HTMLButtonElement>) => {
    event?.preventDefault();
    const next =
      deleteTarget === "single" && editingAnnouncement
        ? announcements.filter((item) => item.id !== editingAnnouncement.id)
        : announcements.filter((item) => !selectedIds.includes(item.id));
    if (next.length === announcements.length) return;
    if (!(await persistAnnouncements(next))) return;
    setSelectedIds([]);
    setShowDeleteDialog(false);
    setEditingAnnouncement(null);
  };

  const handleSubmitForm = async (values: AnnouncementFormValuesFromSchema) => {
    const id = editingAnnouncement
      ? editingAnnouncement.id
      : Math.max(...announcements.map((item) => item.id), 0) + 1;
    const nextAnnouncement = serializeAnnouncement(
      values,
      id,
      editingAnnouncement,
    );
    const next = editingAnnouncement
      ? announcements.map((item) => (item.id === id ? nextAnnouncement : item))
      : [...announcements, nextAnnouncement];
    if (await persistAnnouncements(next)) {
      setShowDialog(false);
      setEditingAnnouncement(null);
    }
  };

  const handleLogoFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    const allowedTypes = new Set([
      "image/png",
      "image/jpeg",
      "image/webp",
      "image/gif",
      "image/svg+xml",
    ]);
    if (!allowedTypes.has(file.type)) {
      toast.error(t("Logo must be a supported image file"));
      return;
    }
    const reader = new FileReader();
    reader.onload = (loadEvent) => {
      const result = loadEvent.target?.result;
      if (typeof result !== "string") return;
      if (result.length > ANNOUNCEMENT_LOGO_MAX_BYTES) {
        toast.error(t("Logo file must be 64 KB or smaller"));
        return;
      }
      form.setValue("logo", result, {
        shouldDirty: true,
        shouldValidate: true,
      });
    };
    reader.readAsDataURL(file);
  };

  const toggleSelectAll = (checked: boolean) => {
    setSelectedIds(checked ? announcements.map((item) => item.id) : []);
  };

  const toggleSelectOne = (id: number, checked: boolean) => {
    setSelectedIds((prev) =>
      checked ? [...prev, id] : prev.filter((item) => item !== id),
    );
  };

  const sortedAnnouncements = useMemo(
    () =>
      [...announcements].sort(
        (a, b) =>
          new Date(b.publishDate).getTime() - new Date(a.publishDate).getTime(),
      ),
    [announcements],
  );

  const getRelativeTime = (date: string) => {
    const diffMs = new Date().getTime() - new Date(date).getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    return `${diffDays}d ago`;
  };

  const logo = form.watch("logo");
  const logoPreview =
    logo && logo.startsWith("/") ? officialWebsiteUrl(logo) : logo;

  return (
    <SettingsSection title={t("Announcements")}>
      <div className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <Button onClick={handleAdd} size="sm">
              <Plus className="mr-2 h-4 w-4" />
              {t("Add Announcement")}
            </Button>
            <Button
              onClick={handleBatchDelete}
              size="sm"
              variant="destructive"
              disabled={selectedIds.length === 0 || updateOption.isPending}
            >
              <Trash2 className="mr-2 h-4 w-4" />
              {t("Delete (")}
              {selectedIds.length})
            </Button>
          </div>
          <SettingsSwitchField
            checked={isEnabled}
            onCheckedChange={handleToggleEnabled}
            label={t("Enabled")}
            className="border-b-0 py-0"
          />
        </div>

        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="w-12">
                  <Checkbox
                    checked={
                      selectedIds.length === announcements.length &&
                      announcements.length > 0
                    }
                    onCheckedChange={toggleSelectAll}
                  />
                </TableHead>
                <TableHead>{t("Content")}</TableHead>
                <TableHead>{t("Publish Date")}</TableHead>
                <TableHead>{t("Type")}</TableHead>
                <TableHead>{t("Extra")}</TableHead>
                <TableHead>{t("Link URL")}</TableHead>
                <TableHead className="w-32">{t("Actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {sortedAnnouncements.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="h-24 text-center">
                    {t(
                      'No announcements yet. Click "Add Announcement" to create one.',
                    )}
                  </TableCell>
                </TableRow>
              ) : (
                sortedAnnouncements.map((announcement) => {
                  const typeOption = typeOptions.find(
                    (option) => option.value === announcement.type,
                  );
                  return (
                    <TableRow key={announcement.id}>
                      <TableCell>
                        <Checkbox
                          checked={selectedIds.includes(announcement.id)}
                          onCheckedChange={(checked) =>
                            toggleSelectOne(announcement.id, checked === true)
                          }
                        />
                      </TableCell>
                      <TableCell
                        className="max-w-xs truncate"
                        title={announcement.content}
                      >
                        {announcement.content}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-col gap-1">
                          <span className="text-sm font-medium">
                            {getRelativeTime(announcement.publishDate)}
                          </span>
                          <span className="text-muted-foreground text-xs">
                            {dayjs(announcement.publishDate).format(
                              "YYYY-MM-DD HH:mm:ss",
                            )}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell>
                        <StatusBadge
                          label={
                            typeOption ? t(typeOption.label) : t("Default")
                          }
                          variant={typeOption?.badgeVariant ?? "neutral"}
                          copyable={false}
                        />
                      </TableCell>
                      <TableCell
                        className="text-muted-foreground max-w-xs truncate"
                        title={announcement.extra}
                      >
                        {announcement.extra || "-"}
                      </TableCell>
                      <TableCell
                        className="text-muted-foreground max-w-xs truncate"
                        title={announcement.link}
                      >
                        {announcement.link || "-"}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-2">
                          <Button
                            onClick={() => handleEdit(announcement)}
                            size="sm"
                            variant="ghost"
                            aria-label={t("Edit")}
                          >
                            <Edit className="h-4 w-4" />
                          </Button>
                          <Button
                            onClick={() => handleDelete(announcement)}
                            size="sm"
                            variant="ghost"
                            aria-label={t("Delete")}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </div>
      </div>

      <Dialog
        open={showDialog}
        onOpenChange={(open) => {
          if (!updateOption.isPending) setShowDialog(open);
        }}
        title={
          editingAnnouncement ? t("Edit Announcement") : t("Add Announcement")
        }
        description={t(
          "Configure announcement copy for every language supported by the website",
        )}
        contentClassName="max-w-3xl"
        contentHeight="auto"
        bodyClassName="space-y-5"
        footer={
          <>
            <Button
              type="button"
              variant="outline"
              disabled={updateOption.isPending}
              onClick={() => setShowDialog(false)}
            >
              {t("Cancel")}
            </Button>
            <Button
              type="submit"
              form={ANNOUNCEMENT_FORM_ID}
              disabled={updateOption.isPending}
            >
              {updateOption.isPending
                ? t("Saving...")
                : editingAnnouncement
                  ? t("Update")
                  : t("Add")}
            </Button>
          </>
        }
      >
        <Form {...form}>
          <form
            id={ANNOUNCEMENT_FORM_ID}
            onSubmit={form.handleSubmit(handleSubmitForm)}
            className="space-y-5"
          >
            <FormField
              control={form.control}
              name="logo"
              render={({ field }) => {
                const selectedLogoValue = BUILT_IN_MODEL_LOGOS.some(
                  (option) => option.value === logo,
                )
                  ? logo
                  : logo
                    ? "custom"
                    : undefined;

                return (
                  <FormItem className="bg-muted/20 rounded-lg border p-3">
                    <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(15rem,auto)] sm:items-center">
                      <div className="flex min-w-0 items-center gap-3">
                        {logo ? (
                          <img
                            src={logoPreview}
                            alt={t("Logo")}
                            className="h-10 w-10 shrink-0 rounded border bg-background object-contain p-1"
                          />
                        ) : (
                          <div className="bg-background text-muted-foreground flex h-10 w-10 shrink-0 items-center justify-center rounded border">
                            <ImageIcon className="h-4 w-4" />
                          </div>
                        )}
                        <div className="min-w-0">
                          <FormLabel>{t("Logo")}</FormLabel>
                          <p className="text-muted-foreground text-xs">
                            {t(
                              "Optional logo shown before the announcement content (64 KB max).",
                            )}
                          </p>
                        </div>
                      </div>

                      <div className="grid min-w-0 gap-2 sm:flex sm:flex-wrap sm:justify-end">
                        <Select
                          items={[
                            { value: "custom", label: t("Custom upload") },
                            ...BUILT_IN_MODEL_LOGOS.map((option) => ({
                              value: option.value,
                              label: option.label,
                            })),
                          ]}
                          value={selectedLogoValue}
                          onValueChange={(value) =>
                            field.onChange(
                              value && value !== "custom" ? value : "",
                            )
                          }
                        >
                          <SelectTrigger
                            aria-label={t("Logo")}
                            className="w-full min-w-0 sm:w-56"
                          >
                            <SelectValue
                              placeholder={t("Choose a model logo")}
                            />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectGroup>
                              <SelectItem value="custom">
                                {t("Custom upload")}
                              </SelectItem>
                              {BUILT_IN_MODEL_LOGOS.map((option) => (
                                <SelectItem
                                  key={option.value}
                                  value={option.value}
                                >
                                  <span className="flex items-center gap-2">
                                    <img
                                      src={officialWebsiteUrl(option.value)}
                                      alt=""
                                      className="h-4 w-4 object-contain"
                                    />
                                    {option.label}
                                  </span>
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                        <FormControl>
                          <input
                            ref={logoInputRef}
                            type="file"
                            accept="image/png,image/jpeg,image/webp,image/gif,image/svg+xml"
                            className="hidden"
                            onChange={handleLogoFileChange}
                          />
                        </FormControl>
                        <div className="flex min-w-0 gap-2">
                          <Button
                            type="button"
                            variant="outline"
                            className="min-w-0 flex-1 sm:flex-none"
                            onClick={() => logoInputRef.current?.click()}
                          >
                            <Upload className="mr-2 h-4 w-4" />
                            {t("Upload")}
                          </Button>
                          {logo ? (
                            <Button
                              type="button"
                              variant="outline"
                              className="min-w-0 flex-1 sm:flex-none"
                              onClick={() => field.onChange("")}
                            >
                              {t("Clear")}
                            </Button>
                          ) : null}
                        </div>
                      </div>
                    </div>
                    <FormMessage />
                  </FormItem>
                );
              }}
            />

            <Tabs
              value={activeLocale}
              onValueChange={(value) => setActiveLocale(value as WebsiteLocale)}
            >
              <TabsList className="flex h-auto w-full max-w-full flex-nowrap justify-start gap-1 overflow-x-auto overflow-y-hidden">
                {WEBSITE_LOCALES.map((locale) => (
                  <TabsTrigger
                    key={locale}
                    value={locale}
                    className="flex-none px-2 py-1"
                  >
                    {WEBSITE_LOCALE_LABELS[locale]}
                  </TabsTrigger>
                ))}
              </TabsList>
              <TabsContent value={activeLocale} className="mt-4 space-y-4">
                <FormField
                  control={form.control}
                  name={localizedPath("content_i18n", activeLocale)}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("Content")} · {WEBSITE_LOCALE_LABELS[activeLocale]}
                      </FormLabel>
                      <FormControl>
                        <Textarea
                          placeholder={t(
                            "Enter announcement content (supports Markdown/HTML)",
                          )}
                          rows={4}
                          {...field}
                          value={String(field.value ?? "")}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          "Maximum 500 characters. Supports Markdown and HTML.",
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name={localizedPath("intro_i18n", activeLocale)}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("Intro / summary (Optional)")} ·{" "}
                        {WEBSITE_LOCALE_LABELS[activeLocale]}
                      </FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t("Additional information")}
                          {...field}
                          value={String(field.value ?? "")}
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          "Optional supplementary information (max 200 characters)",
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name={localizedPath("link_label_i18n", activeLocale)}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t("Link text")} · {WEBSITE_LOCALE_LABELS[activeLocale]}
                      </FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t("Optional call-to-action text")}
                          {...field}
                          value={String(field.value ?? "")}
                        />
                      </FormControl>
                      <FormDescription>
                        {t("This text is shown for the announcement link.")}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </TabsContent>
            </Tabs>

            <FormField
              control={form.control}
              name="publishDate"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("Publish Date")}</FormLabel>
                  <FormControl>
                    <DateTimePicker
                      value={field.value ? new Date(field.value) : undefined}
                      onChange={(date) =>
                        field.onChange(date ? date.toISOString() : "")
                      }
                      placeholder={t("Select publish date")}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      "Date and time when this announcement should be displayed",
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="type"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("Type")}</FormLabel>
                  <Select value={field.value} onValueChange={field.onChange}>
                    <FormControl>
                      <SelectTrigger className="w-full">
                        <SelectValue
                          placeholder={t("Select announcement type")}
                        />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectGroup>
                        {typeOptions.map((option) => (
                          <SelectItem key={option.value} value={option.value}>
                            <div className="flex items-center gap-2">
                              <div
                                className={`h-3 w-3 rounded-full ${option.color}`}
                              />
                              {t(option.label)}
                            </div>
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name="link"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t("Link URL")}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t("Optional destination URL")}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      "Use a site path such as /pricing or a full https:// URL.",
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

          </form>
        </Form>
      </Dialog>

      <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("Are you sure?")}</AlertDialogTitle>
            <AlertDialogDescription>
              {deleteTarget === "single"
                ? t("This announcement will be removed from the list.")
                : t("{{count}} announcements will be removed from the list.", {
                    count: selectedIds.length,
                  })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={updateOption.isPending}>
              {t("Cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={updateOption.isPending}
              onClick={confirmDelete}
            >
              {updateOption.isPending ? t("Saving...") : t("Delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  );
}
