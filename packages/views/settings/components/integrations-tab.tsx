"use client";

import { useQuery } from "@tanstack/react-query";
import { LarkTab } from "./lark-tab";
import { SlackTab } from "./slack-tab";
import { ComposioTab } from "./composio-tab";
import { WikiSettingsTab } from "../../wiki/components/wiki-settings-tab";
import { OssSettingsTab } from "./oss-tab";
import { ApiError } from "@multica/core/api";
import { composioToolkitsOptions } from "@multica/core/composio";
import { useFeatureEnabled } from "@multica/core/config";
import { COMPOSIO_MCP_APPS_FLAG } from "@multica/core/feature-flags";
import { useT } from "../../i18n";

// Integrations is the umbrella tab for third-party platform connections.
// GitHub has its own top-level tab (see github-tab.tsx); everything else
// — currently Lark, Composio, with Slack/Linear etc. to follow — lives in
// here under its own section heading so additional integrations slot in
// without changing the IA. IntegrationsTab is just the host; each
// integration owns its own description and install flow.
export function IntegrationsTab() {
  const { t } = useT("settings");

  const composioEnabled = useFeatureEnabled(COMPOSIO_MCP_APPS_FLAG, false);
  // Composio is hidden entirely until the feature is enabled and a key is
  // configured server-side. A 503 from the toolkits endpoint means the server
  // withheld the integration despite the frontend flag being on.
  const composioToolkits = useQuery({
    ...composioToolkitsOptions(),
    enabled: composioEnabled,
  });
  const composioUnconfigured =
    composioToolkits.error instanceof ApiError && composioToolkits.error.status === 503;

  return (
    <div className="space-y-10">
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">Wiki</h2>
        <WikiSettingsTab />
      </section>

      <section className="space-y-4">
        <h2 className="text-sm font-semibold">OSS · 对象存储</h2>
        <OssSettingsTab />
      </section>

      <section className="space-y-4">
        <h2 className="text-sm font-semibold">{t(($) => $.slack.section_title)}</h2>
        <SlackTab />
      </section>
      <section className="space-y-4">
        <h2 className="text-sm font-semibold">{t(($) => $.lark.section_title)}</h2>
        <LarkTab />
      </section>
      {composioEnabled && (
        <section className="space-y-4">
          <h2 className="text-sm font-semibold">{t(($) => $.composio.section_title)}</h2>
          {composioUnconfigured ? (
            <div className="text-sm text-muted-foreground space-y-1">
              <p className="font-medium">{t(($) => $.composio.not_enabled_title)}</p>
              <p>
                {t(($) => $.composio.not_enabled_description_prefix)}{" "}
                <code className="text-xs bg-muted px-1 py-0.5 rounded">COMPOSIO_API_KEY</code>{" "}
                {t(($) => $.composio.not_enabled_description_suffix)}
              </p>
            </div>
          ) : (
            <ComposioTab />
          )}
        </section>
      )}
    </div>
  );
}
