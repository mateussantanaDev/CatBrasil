export function cpfDigits(value) {
    return String(value || '').replace(/\D/g, '');
}

export function formatCpf(value) {
    const digits = cpfDigits(value).slice(0, 11);
    return digits.replace(/^(\d{3})(\d)/, '$1.$2')
        .replace(/^(\d{3})\.(\d{3})(\d)/, '$1.$2.$3')
        .replace(/^(\d{3})\.(\d{3})\.(\d{3})(\d)/, '$1.$2.$3-$4');
}

export function isValidCpf(value) {
    const digits = cpfDigits(value);
    if (digits.length !== 11 || /^(\d)\1{10}$/.test(digits)) return false;

    for (let length = 9; length < 11; length++) {
        const sum = digits.slice(0, length).split('').reduce((total, digit, index) =>
            total + Number(digit) * (length + 1 - index), 0);
        const expected = (sum * 10) % 11 % 10;
        if (expected !== Number(digits[length])) return false;
    }
    return true;
}

export function isValidIdentityName(value) {
    const name = String(value || '').trim();
    return name.length >= 5 && name.length <= 120 && name.split(/\s+/).length >= 2 && !/[\d\p{Cc}]/u.test(name);
}

export function isValidEmail(value) {
    const email = String(value || '').trim();
    return email.length <= 254 && /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}
