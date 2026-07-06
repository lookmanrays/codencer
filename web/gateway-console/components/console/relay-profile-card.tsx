import { MoreHorizontal } from "lucide-react";
import { useDeleteRelayProfile } from "@/api/relays";
import { RelayHealthBadge } from "@/components/console/relay-health-badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Alert } from "@/components/ui/alert";
import { KeyValueList } from "@/components/ui/key-value-list";
import { SecretField } from "@/components/ui/secret-field";
import type { RelayProfile } from "@/schemas/relays";

export function RelayProfileCard({ relay }: { relay: RelayProfile }) {
  const deleteRelay = useDeleteRelayProfile();
  return (
    <Card>
      <CardHeader>
        <div className="flex min-w-0 items-start justify-between gap-md">
          <div className="min-w-0">
            <CardTitle>{relay.name}</CardTitle>
            <p className="m-0 mt-xs text-body-sm text-ink-secondary">
              {relay.type === "managed"
                ? "Default Relay profile"
                : "User-added backend Relay profile"}
            </p>
          </div>
          <RelayHealthBadge relay={relay} />
        </div>
      </CardHeader>
      <CardContent className="grid min-w-0 gap-md">
        {deleteRelay.error ? (
          <Alert title="Relay profile was not removed" tone="danger">
            {deleteRelay.error.message}
          </Alert>
        ) : null}
        <KeyValueList
          items={[
            { label: "ID", value: relay.id },
            { label: "Type", value: relay.type },
            { label: "URL", value: relay.url },
            { label: "Enabled", value: relay.enabled ? "yes" : "no" },
          ]}
        />
        <SecretField label="Token reference" value={relay.tokenRef} />
      </CardContent>
      <CardFooter className="flex min-w-0 flex-wrap justify-end gap-sm">
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button
              disabled={relay.type === "managed" || deleteRelay.isPending}
              variant="danger"
            >
              Remove
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogTitle className="m-0 text-h3 font-bold">
              Remove Relay profile?
            </AlertDialogTitle>
            <AlertDialogDescription className="mt-sm text-body text-ink-secondary">
              Gateway will remove this Relay profile from the current workspace
              registry. Managed default Relay profiles cannot be removed here.
            </AlertDialogDescription>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                disabled={deleteRelay.isPending}
                onClick={() => deleteRelay.mutate(relay.id)}
              >
                <MoreHorizontal className="mr-sm h-4 w-4" /> Remove
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </CardFooter>
    </Card>
  );
}
