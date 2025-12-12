import React from "react";

import { Card } from "@/components/ui/card";

interface InfographicProps {
  icon?: React.ReactNode;
  label: string;
  value: number;
}

export function Infographic({
  icon,
  label = "Label",
  value = 0,
}: InfographicProps) {
  return (
    <Card className="aspect-2/1 max-w-50 min-w-30 p-2 content-contain gap-2 place-content-center">
      <div className="flex justify-end">
        <h1 className="text-[0.65rem]"> {label} </h1>
      </div>
      <div className="grid grid-cols-3 pb-4">
        <div className="grid place-content-center">{icon}</div>
        <div className="grid col-span-2 text-5xl justify-self-center">
          <p> {value} </p>
        </div>
      </div>
    </Card>
  );
}
