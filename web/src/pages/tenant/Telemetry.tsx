import { useState } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Badge } from '@/components/ui/badge';

interface TelemetryConfig {
  enabled: boolean;
  prometheusEnabled: boolean;
  prometheusEndpoint: string;
  otlpEnabled: boolean;
  otlpEndpoint: string;
  logLevel: string;
  sampleRate: number;
}

export default function Telemetry() {
  const [config, setConfig] = useState<TelemetryConfig>({
    enabled: true,
    prometheusEnabled: true,
    prometheusEndpoint: '/metrics',
    otlpEnabled: false,
    otlpEndpoint: '',
    logLevel: 'info',
    sampleRate: 100,
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
        <h1 className="text-3xl font-bold tracking-tight">Telemetry</h1>
        <p className="text-muted-foreground">Configure metrics, tracing, and observability</p>
      </div>

      <div className="grid gap-6">
        {/* Main Toggle */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              Telemetry
              <Badge className={config.enabled ? 'bg-green-500' : 'bg-gray-500'}>
                {config.enabled ? 'Enabled' : 'Disabled'}
              </Badge>
            </CardTitle>
            <CardDescription>Enable or disable all telemetry collection</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-center justify-between">
              <div className="space-y-0.5">
                <label className="text-sm font-medium">Enable Telemetry</label>
                <p className="text-sm text-muted-foreground">
                  Collect metrics, traces, and logs for observability
                </p>
              </div>
              <Switch
                checked={config.enabled}
                onCheckedChange={(checked) => setConfig({ ...config, enabled: checked })}
              />
            </div>
          </CardContent>
        </Card>

        {config.enabled && (
          <>
            {/* Prometheus */}
            <Card>
              <CardHeader>
                <CardTitle>Prometheus Metrics</CardTitle>
                <CardDescription>Export metrics in Prometheus format</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="space-y-0.5">
                    <label className="text-sm font-medium">Enable Prometheus</label>
                    <p className="text-sm text-muted-foreground">
                      Expose a /metrics endpoint for Prometheus scraping
                    </p>
                  </div>
                  <Switch
                    checked={config.prometheusEnabled}
                    onCheckedChange={(checked) =>
                      setConfig({ ...config, prometheusEnabled: checked })
                    }
                  />
                </div>
                {config.prometheusEnabled && (
                  <div className="pt-4 space-y-2">
                    <label className="text-sm font-medium">Metrics Endpoint</label>
                    <Input
                      value={config.prometheusEndpoint}
                      onChange={(e) =>
                        setConfig({ ...config, prometheusEndpoint: e.target.value })
                      }
                    />
                  </div>
                )}
              </CardContent>
            </Card>

            {/* OpenTelemetry */}
            <Card>
              <CardHeader>
                <CardTitle>OpenTelemetry (OTLP)</CardTitle>
                <CardDescription>Send traces and metrics via OTLP</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center justify-between">
                  <div className="space-y-0.5">
                    <label className="text-sm font-medium">Enable OTLP Export</label>
                    <p className="text-sm text-muted-foreground">
                      Send telemetry data to an OTLP-compatible backend
                    </p>
                  </div>
                  <Switch
                    checked={config.otlpEnabled}
                    onCheckedChange={(checked) => setConfig({ ...config, otlpEnabled: checked })}
                  />
                </div>
                {config.otlpEnabled && (
                  <div className="pt-4 space-y-2">
                    <label className="text-sm font-medium">OTLP Endpoint</label>
                    <Input
                      placeholder="http://localhost:4317"
                      value={config.otlpEndpoint}
                      onChange={(e) => setConfig({ ...config, otlpEndpoint: e.target.value })}
                    />
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Sampling */}
            <Card>
              <CardHeader>
                <CardTitle>Sampling</CardTitle>
                <CardDescription>Configure trace sampling rate</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <label className="text-sm font-medium">Sample Rate (%)</label>
                  <div className="flex items-center gap-4">
                    <Input
                      type="number"
                      min="0"
                      max="100"
                      value={config.sampleRate}
                      onChange={(e) =>
                        setConfig({ ...config, sampleRate: parseInt(e.target.value) || 0 })
                      }
                      className="max-w-[120px]"
                    />
                    <span className="text-sm text-muted-foreground">
                      {config.sampleRate === 100
                        ? 'All requests'
                        : config.sampleRate === 0
                        ? 'No requests'
                        : `1 in ${Math.round(100 / config.sampleRate)} requests`}
                    </span>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* Log Level */}
            <Card>
              <CardHeader>
                <CardTitle>Logging</CardTitle>
                <CardDescription>Configure log verbosity</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="space-y-2">
                  <label className="text-sm font-medium">Log Level</label>
                  <div className="flex gap-2">
                    {['debug', 'info', 'warn', 'error'].map((level) => (
                      <Button
                        key={level}
                        variant={config.logLevel === level ? 'default' : 'outline'}
                        size="sm"
                        onClick={() => setConfig({ ...config, logLevel: level })}
                      >
                        {level.charAt(0).toUpperCase() + level.slice(1)}
                      </Button>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>
          </>
        )}
      </div>

      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={isSaving}>
          {isSaving ? 'Saving...' : 'Save Configuration'}
        </Button>
      </div>
    </div>
  );
}

