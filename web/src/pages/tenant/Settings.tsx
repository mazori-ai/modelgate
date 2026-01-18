import { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';

interface TenantSettings {
  rateLimitEnabled: boolean;
  rateLimitRpm: number;
  rateLimitTpm: number;
  loggingEnabled: boolean;
  logRetentionDays: number;
  webhookUrl: string;
  defaultModel: string;
}

export default function Settings() {
  const [settings, setSettings] = useState<TenantSettings>({
    rateLimitEnabled: true,
    rateLimitRpm: 60,
    rateLimitTpm: 100000,
    loggingEnabled: true,
    logRetentionDays: 30,
    webhookUrl: '',
    defaultModel: 'gpt-4o',
  });
  const [isSaving, setIsSaving] = useState(false);

  const handleSave = async () => {
    setIsSaving(true);
    // TODO: Call GraphQL mutation
    await new Promise(resolve => setTimeout(resolve, 500));
    setIsSaving(false);
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Settings</h1>
        <p className="text-muted-foreground">Configure tenant settings and preferences</p>
      </div>

      <div className="grid gap-6">
        {/* Rate Limiting */}
        <Card>
          <CardHeader>
            <CardTitle>Rate Limiting</CardTitle>
            <CardDescription>Configure rate limits for API requests</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <label className="text-sm font-medium">Enable Rate Limiting</label>
                <p className="text-sm text-muted-foreground">
                  Limit the number of requests per minute
                </p>
              </div>
              <Switch
                checked={settings.rateLimitEnabled}
                onCheckedChange={(checked) =>
                  setSettings({ ...settings, rateLimitEnabled: checked })
                }
              />
            </div>
            {settings.rateLimitEnabled && (
              <div className="grid grid-cols-2 gap-4 pt-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Requests per Minute (RPM)</label>
                  <Input
                    type="number"
                    value={settings.rateLimitRpm}
                    onChange={(e) =>
                      setSettings({ ...settings, rateLimitRpm: parseInt(e.target.value) || 0 })
                    }
                  />
                </div>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Tokens per Minute (TPM)</label>
                  <Input
                    type="number"
                    value={settings.rateLimitTpm}
                    onChange={(e) =>
                      setSettings({ ...settings, rateLimitTpm: parseInt(e.target.value) || 0 })
                    }
                  />
                </div>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Logging */}
        <Card>
          <CardHeader>
            <CardTitle>Logging</CardTitle>
            <CardDescription>Configure request logging and retention</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <label className="text-sm font-medium">Enable Request Logging</label>
                <p className="text-sm text-muted-foreground">
                  Log all API requests for analytics and debugging
                </p>
              </div>
              <Switch
                checked={settings.loggingEnabled}
                onCheckedChange={(checked) =>
                  setSettings({ ...settings, loggingEnabled: checked })
                }
              />
            </div>
            {settings.loggingEnabled && (
              <div className="pt-4 space-y-2">
                <label className="text-sm font-medium">Log Retention (days)</label>
                <Input
                  type="number"
                  value={settings.logRetentionDays}
                  onChange={(e) =>
                    setSettings({ ...settings, logRetentionDays: parseInt(e.target.value) || 0 })
                  }
                  className="max-w-[200px]"
                />
              </div>
            )}
          </CardContent>
        </Card>

        {/* Webhooks */}
        <Card>
          <CardHeader>
            <CardTitle>Webhooks</CardTitle>
            <CardDescription>Configure webhook notifications</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Webhook URL</label>
              <Input
                placeholder="https://your-server.com/webhook"
                value={settings.webhookUrl}
                onChange={(e) => setSettings({ ...settings, webhookUrl: e.target.value })}
              />
              <p className="text-sm text-muted-foreground">
                Receive notifications for budget alerts and other events
              </p>
            </div>
          </CardContent>
        </Card>

        {/* Default Model */}
        <Card>
          <CardHeader>
            <CardTitle>Default Model</CardTitle>
            <CardDescription>Set the default model for API requests</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Default Model</label>
              <Input
                placeholder="gpt-4o"
                value={settings.defaultModel}
                onChange={(e) => setSettings({ ...settings, defaultModel: e.target.value })}
              />
              <p className="text-sm text-muted-foreground">
                Used when no model is specified in the API request
              </p>
            </div>
          </CardContent>
        </Card>
      </div>

      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={isSaving}>
          {isSaving ? 'Saving...' : 'Save Settings'}
        </Button>
      </div>
    </div>
  );
}

