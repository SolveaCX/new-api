/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

export function ReportQueryError(props: { hasData: boolean }) {
  const { t } = useTranslation()
  return (
    <Alert variant='destructive'>
      <AlertTitle>{t('Unable to load supply chain report')}</AlertTitle>
      <AlertDescription>
        {props.hasData
          ? t(
              'Some report sections could not be loaded. Do not treat the visible totals as complete.'
            )
          : t('Please try again later.')}
      </AlertDescription>
    </Alert>
  )
}
