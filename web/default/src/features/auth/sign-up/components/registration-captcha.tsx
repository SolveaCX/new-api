/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or (at your
option) any later version.
*/
import { useEffect, useRef, useState } from 'react'
import { CircleArrowReload01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Slider } from '@/components/ui/slider'
import { Spinner } from '@/components/ui/spinner'
import {
  getRegistrationCaptcha,
  verifyRegistrationCaptcha,
} from '@/features/auth/api'
import type { RegistrationCaptchaChallenge } from '@/features/auth/types'

type RegistrationCaptchaProps = {
  onVerified: (token: string) => void
}

export function RegistrationCaptcha({ onVerified }: RegistrationCaptchaProps) {
  const { t } = useTranslation()
  const [challenge, setChallenge] =
    useState<RegistrationCaptchaChallenge | null>(null)
  const [answer, setAnswer] = useState(0)
  const [version, setVersion] = useState(0)
  const [isLoading, setIsLoading] = useState(true)
  const [isVerifying, setIsVerifying] = useState(false)
  const [error, setError] = useState('')
  const generationRef = useRef(0)
  const verifyingRef = useRef(false)

  useEffect(() => {
    const generation = ++generationRef.current
    void getRegistrationCaptcha('slide')
      .then((response) => {
        if (generation !== generationRef.current) return
        if (!response.success || response.data?.type !== 'slide') {
          throw new Error(response.message)
        }
        setChallenge(response.data)
        setAnswer(response.data.start_x || 0)
      })
      .catch(() => {
        if (generation !== generationRef.current) return
        setError(t('Unable to load verification. Please try again.'))
      })
      .finally(() => {
        if (generation === generationRef.current) setIsLoading(false)
      })
    return () => {
      // Ignore late responses after the dialog closes or the challenge changes.
      generationRef.current = generation + 1
    }
  }, [version, t])

  function handleRefresh() {
    if (isLoading || verifyingRef.current) return
    generationRef.current++
    setChallenge(null)
    setError('')
    setIsLoading(true)
    setVersion((current) => current + 1)
  }

  async function handleVerify(value: number) {
    if (!challenge || isLoading || verifyingRef.current) return
    const generation = generationRef.current
    verifyingRef.current = true
    setIsVerifying(true)
    setError('')
    try {
      const response = await verifyRegistrationCaptcha({
        id: challenge.id,
        x: value,
        y: challenge.start_y || 0,
      })
      if (generation !== generationRef.current) return
      if (!response.success || !response.data?.token) {
        throw new Error(response.message)
      }
      onVerified(response.data.token)
    } catch (_error) {
      if (generation !== generationRef.current) return
      // Each answer consumes its challenge; a failed attempt needs a new one.
      setChallenge(null)
      setIsLoading(true)
      setError(t('Verification failed. A new challenge has been loaded.'))
      setVersion((current) => current + 1)
    } finally {
      verifyingRef.current = false
      if (generation === generationRef.current) setIsVerifying(false)
    }
  }

  const slideMax = Math.max(
    0,
    (challenge?.width || 0) - (challenge?.piece_width || 0)
  )

  return (
    <div className='flex flex-col gap-4' aria-busy={isLoading || isVerifying}>
      {isLoading ? <Skeleton className='aspect-[5/3] w-full' /> : null}

      {!isLoading && challenge ? (
        <>
          <div
            className='bg-muted relative mx-auto w-full max-w-[300px] overflow-hidden rounded-lg border'
            style={{ aspectRatio: `${challenge.width} / ${challenge.height}` }}
          >
            <img
              src={challenge.background_image}
              alt={t('Slide puzzle verification image')}
              className='size-full object-cover'
              draggable={false}
            />
            {challenge.piece_image ? (
              <img
                src={challenge.piece_image}
                alt=''
                aria-hidden='true'
                className='pointer-events-none absolute select-none'
                style={{
                  left: `${(answer / challenge.width) * 100}%`,
                  top: `${((challenge.start_y || 0) / challenge.height) * 100}%`,
                  width: `${((challenge.piece_width || 0) / challenge.width) * 100}%`,
                  height: `${((challenge.piece_height || 0) / challenge.height) * 100}%`,
                }}
                draggable={false}
              />
            ) : null}
          </div>
          <Slider
            value={[answer]}
            min={0}
            max={slideMax}
            step={1}
            onValueChange={(values) =>
              setAnswer(typeof values === 'number' ? values : values[0] || 0)
            }
            onValueCommitted={(values) =>
              void handleVerify(
                typeof values === 'number' ? values : values[0] || 0
              )
            }
            disabled={isVerifying}
            aria-label={t('Drag the puzzle piece')}
            className='py-3'
          />
        </>
      ) : null}

      {error ? (
        <Alert variant='destructive'>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      ) : null}

      <div className='flex items-center justify-between gap-2'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={handleRefresh}
          disabled={isLoading || isVerifying}
          aria-label={t('Refresh challenge')}
        >
          <HugeiconsIcon
            icon={CircleArrowReload01Icon}
            data-icon='inline-start'
          />
          {t('Refresh')}
        </Button>
        {isVerifying ? <Spinner /> : null}
      </div>
    </div>
  )
}
