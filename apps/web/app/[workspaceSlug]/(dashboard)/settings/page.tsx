"use client";

import { Library } from "lucide-react";
import { SettingsPage, TemplateLibraryPage } from "@multica/views/settings";

export default function Page() {
  return (
    <SettingsPage
      extraAccountTabs={[
        {
          value: "template-library",
          label: "Template Library",
          icon: Library,
          content: <TemplateLibraryPage />,
        },
      ]}
    />
  );
}
