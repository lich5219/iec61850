//go:build windows && amd64

package iec61850

/*
#cgo CFLAGS: -I./libiec61850/inc/hal/inc -I./libiec61850/inc/common/inc -I./libiec61850/inc/goose -I./libiec61850/inc/sampled_values -I./libiec61850/inc/iec61850/inc -I./libiec61850/inc/iec61850/inc_private -I./libiec61850/inc/logging -I./libiec61850/inc/mms/inc -I./libiec61850/inc/mms/inc_private -I./libiec61850/inc/mms/iso_mms/asn1c
#cgo LDFLAGS: -static-libgcc -static-libstdc++ -Wl,--allow-multiple-definition -L${SRCDIR}/libiec61850/lib/win64 -liec61850 -lhal -lwpcap -lpacket -lws2_32 -liphlpapi

#include <stdarg.h>
#include <stddef.h>

int _snprintf(char* buffer, size_t count, const char* format, ...)
{
	va_list args;
	int ret;

	va_start(args, format);
	ret = __builtin_vsnprintf(buffer, count, format, args);
	va_end(args);

	return ret;
}
*/
import "C"
