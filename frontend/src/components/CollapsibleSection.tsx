import * as React from "react";

import { ChevronsUpDown } from "lucide-react";

import { Button, buttonVariants } from "@/components/ui/button";
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";

interface CollapsibleSectionProps {
  label: string;
  children?: React.ReactNode;
}

export function CollapsibleSection({
  label,
  children,
}: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = React.useState(false);
  return (
    <Collapsible open={isOpen} onOpenChange={setIsOpen} className="m-4">
      <Card className="p-2">
        <CardHeader className="border-b p-2 [.border-b]:pb-2">
          <CardTitle className="row-span-2 self-center">{label}</CardTitle>
          <CardAction>
            <CollapsibleTrigger
              className={buttonVariants({ variant: "ghost", size: "icon" })}
            >
              <ChevronsUpDown />
            </CollapsibleTrigger>
          </CardAction>
        </CardHeader>
        <CollapsibleContent>
          <CardContent>{children}</CardContent>
        </CollapsibleContent>
      </Card>
    </Collapsible>
  );
}
