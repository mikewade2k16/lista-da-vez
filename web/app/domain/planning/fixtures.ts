import type {
  PlanningOperatingDay,
  PlanningFixtures,
  PlanningStaffMember,
  PlanningStaffReference,
  PlanningStore,
  PlanningStoreReference,
  StoreLocationType,
  WeekdayId,
} from './types'
import { WEEKDAY_DEFINITIONS } from './types'

function operatingHours(
  weekdays: Partial<Record<WeekdayId, { opensAt: string; closesAt: string }>>,
): PlanningOperatingDay[] {
  return WEEKDAY_DEFINITIONS.map(({ id }) => ({
    weekday: id,
    isOpen: Boolean(weekdays[id]),
    opensAt: weekdays[id]?.opensAt || '09:00',
    closesAt: weekdays[id]?.closesAt || '18:00',
  }))
}

function operatingHoursByLocationType() {
  return {
    shopping: operatingHours({
      mon: { opensAt: '10:00', closesAt: '22:00' },
      tue: { opensAt: '10:00', closesAt: '22:00' },
      wed: { opensAt: '10:00', closesAt: '22:00' },
      thu: { opensAt: '10:00', closesAt: '22:00' },
      fri: { opensAt: '10:00', closesAt: '22:00' },
      sat: { opensAt: '10:00', closesAt: '22:00' },
      sun: { opensAt: '12:00', closesAt: '20:00' },
    }),
    street: operatingHours({
      mon: { opensAt: '09:00', closesAt: '19:00' },
      tue: { opensAt: '09:00', closesAt: '19:00' },
      wed: { opensAt: '09:00', closesAt: '19:00' },
      thu: { opensAt: '09:00', closesAt: '19:00' },
      fri: { opensAt: '09:00', closesAt: '19:00' },
      sat: { opensAt: '09:00', closesAt: '15:00' },
    }),
  }
}

function shoppingShiftTemplates() {
  return [
    { id: 'opening' as const, name: 'Abertura', startsAt: '10:00', endsAt: '19:00' },
    { id: 'middle' as const, name: 'Intermediário', startsAt: '11:30', endsAt: '20:30' },
    { id: 'closing' as const, name: 'Fechamento', startsAt: '13:00', endsAt: '22:00' },
  ]
}

function streetShiftTemplates() {
  return [
    { id: 'opening' as const, name: 'Abertura', startsAt: '09:00', endsAt: '18:00' },
    { id: 'middle' as const, name: 'Intermediário', startsAt: '09:30', endsAt: '18:30' },
    { id: 'closing' as const, name: 'Fechamento', startsAt: '10:00', endsAt: '19:00' },
  ]
}

function shiftTemplatesByLocationType() {
  return {
    shopping: shoppingShiftTemplates(),
    street: streetShiftTemplates(),
  }
}

function coverageByLocationType() {
  return {
    shopping: {
      enabled: true,
      openingMinimum: 2,
      peakMinimum: 4,
      closingMinimum: 3,
      peakStartsAt: '14:00',
      peakEndsAt: '18:00',
    },
    street: {
      enabled: true,
      openingMinimum: 2,
      peakMinimum: 4,
      closingMinimum: 3,
      peakStartsAt: '12:00',
      peakEndsAt: '15:00',
    },
  }
}

export function createPlanningStore(reference: PlanningStoreReference): PlanningStore {
  const locationType: StoreLocationType = reference.storeType === 'shopping' ? 'shopping' : 'street'
  return {
    id: reference.id,
    name: reference.name,
    city: reference.city,
    timezone: 'America/Maceio',
    locationType,
    goalsByMonth: {},
    operatingHoursByLocationType: operatingHoursByLocationType(),
    shiftTemplatesByLocationType: shiftTemplatesByLocationType(),
    coverageByLocationType: coverageByLocationType(),
    holidays: [],
    exceptions: [],
  }
}

export function createPlanningStaffMember(reference: PlanningStaffReference): PlanningStaffMember {
  return {
    id: reference.id,
    storeId: reference.storeId,
    name: reference.name,
    nick: reference.nick || reference.name,
    employeeCode: reference.employeeCode || '',
    jobRole: reference.role,
    weeklyHours: 44,
    maxDailyHours: 8,
    availableDays: WEEKDAY_DEFINITIONS.map((day) => day.id),
    targetWeight: 1,
    active: true,
    worksSundays: true,
    alternateSundays: false,
    sundayRotationOffset: 0,
    worksHolidays: true,
  }
}

export function createPlanningFixtures(): PlanningFixtures {
  return {
    stores: [
      {
        id: 'store-shopping-jardins',
        name: 'Pérola Jardins',
        city: 'Aracaju',
        timezone: 'America/Maceio',
        locationType: 'shopping',
        goalsByMonth: {
          '2026-07': { month: 420000, p1: 95000, p2: 100000, p3: 105000, p4: 120000, p5: 0 },
          '2026-06': { month: 390000, p1: 90000, p2: 92000, p3: 96000, p4: 112000, p5: 0 },
        },
        operatingHoursByLocationType: operatingHoursByLocationType(),
        shiftTemplatesByLocationType: shiftTemplatesByLocationType(),
        coverageByLocationType: coverageByLocationType(),
        holidays: [],
        exceptions: [],
      },
      {
        id: 'store-street-centro',
        name: 'Pérola Centro',
        city: 'Aracaju',
        timezone: 'America/Maceio',
        locationType: 'street',
        goalsByMonth: {
          '2026-07': { month: 260000, p1: 58000, p2: 62000, p3: 65000, p4: 75000, p5: 0 },
          '2026-06': { month: 245000, p1: 55000, p2: 58000, p3: 61000, p4: 71000, p5: 0 },
        },
        operatingHoursByLocationType: operatingHoursByLocationType(),
        shiftTemplatesByLocationType: shiftTemplatesByLocationType(),
        coverageByLocationType: coverageByLocationType(),
        holidays: [],
        exceptions: [],
      },
    ],
    staff: [
      {
        id: 'staff-ana',
        storeId: 'store-shopping-jardins',
        name: 'Ana Beatriz',
        employeeCode: 'C-1042',
        jobRole: 'Consultora sênior',
        weeklyHours: 44,
        maxDailyHours: 8,
        availableDays: ['mon', 'tue', 'wed', 'thu', 'fri', 'sat'],
        targetWeight: 1.15,
        active: true,
        worksSundays: true,
        alternateSundays: true,
        sundayRotationOffset: 0,
        worksHolidays: true,
      },
      {
        id: 'staff-bruno',
        storeId: 'store-shopping-jardins',
        name: 'Bruno Lima',
        employeeCode: 'C-1061',
        jobRole: 'Consultor',
        weeklyHours: 36,
        maxDailyHours: 8,
        availableDays: ['tue', 'wed', 'thu', 'fri', 'sat', 'sun'],
        targetWeight: 1,
        active: true,
        worksSundays: true,
        alternateSundays: true,
        sundayRotationOffset: 1,
        worksHolidays: true,
      },
      {
        id: 'staff-carla',
        storeId: 'store-shopping-jardins',
        name: 'Carla Souza',
        employeeCode: 'C-1074',
        jobRole: 'Consultora',
        weeklyHours: 30,
        maxDailyHours: 6,
        availableDays: ['mon', 'wed', 'thu', 'fri', 'sat'],
        targetWeight: 0.9,
        active: true,
        worksSundays: false,
        alternateSundays: false,
        sundayRotationOffset: 0,
        worksHolidays: false,
      },
      {
        id: 'staff-diego',
        storeId: 'store-shopping-jardins',
        name: 'Diego Alves',
        employeeCode: 'G-1010',
        jobRole: 'Gerente de loja',
        weeklyHours: 44,
        maxDailyHours: 8,
        availableDays: ['mon', 'tue', 'wed', 'thu', 'fri', 'sun'],
        targetWeight: 0.7,
        active: true,
        worksSundays: true,
        alternateSundays: false,
        sundayRotationOffset: 0,
        worksHolidays: true,
      },
      {
        id: 'staff-elis',
        storeId: 'store-street-centro',
        name: 'Elis Rocha',
        employeeCode: 'C-1092',
        jobRole: 'Consultora sênior',
        weeklyHours: 44,
        maxDailyHours: 8,
        availableDays: ['mon', 'tue', 'wed', 'thu', 'fri', 'sat'],
        targetWeight: 1.1,
        active: true,
        worksSundays: false,
        alternateSundays: false,
        sundayRotationOffset: 0,
        worksHolidays: true,
      },
      {
        id: 'staff-fabio',
        storeId: 'store-street-centro',
        name: 'Fábio Nunes',
        employeeCode: 'C-1108',
        jobRole: 'Consultor',
        weeklyHours: 36,
        maxDailyHours: 8,
        availableDays: ['mon', 'tue', 'wed', 'thu', 'fri'],
        targetWeight: 1,
        active: true,
        worksSundays: false,
        alternateSundays: false,
        sundayRotationOffset: 0,
        worksHolidays: false,
      },
      {
        id: 'staff-gabi',
        storeId: 'store-street-centro',
        name: 'Gabriela Melo',
        employeeCode: 'G-1021',
        jobRole: 'Gerente de loja',
        weeklyHours: 40,
        maxDailyHours: 8,
        availableDays: ['mon', 'tue', 'wed', 'thu', 'fri', 'sat'],
        targetWeight: 0.75,
        active: true,
        worksSundays: false,
        alternateSundays: false,
        sundayRotationOffset: 0,
        worksHolidays: true,
      },
    ],
    policies: [
      {
        id: 'retail-6x1',
        label: 'Varejo 6×1',
        maxDailyHours: 8,
        maxConsecutiveDays: 6,
        minDaysOff: 1,
        breakAfterHours: 6,
        minBreakMinutes: 60,
      },
      {
        id: 'retail-balanced',
        label: 'Semana equilibrada',
        maxDailyHours: 8,
        maxConsecutiveDays: 5,
        minDaysOff: 2,
        breakAfterHours: 6,
        minBreakMinutes: 60,
      },
      {
        id: 'retail-flex',
        label: 'Cobertura flexível',
        maxDailyHours: 10,
        maxConsecutiveDays: 6,
        minDaysOff: 1,
        breakAfterHours: 6,
        minBreakMinutes: 45,
      },
    ],
  }
}
