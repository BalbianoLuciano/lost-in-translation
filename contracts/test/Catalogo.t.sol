// SPDX-License-Identifier: MIT
pragma solidity 0.8.30;

import {Test} from "forge-std/Test.sol";

import {Desplegar} from "../script/Desplegar.s.sol";
import {Distinciones} from "../src/Distinciones.sol";
import {Pilar} from "../src/Pilar.sol";

/// @title El catálogo que se va a desplegar de verdad.
///
/// @notice Los otros tests usan un catálogo inventado, que sirve para probar el
///         contrato pero no dice nada de las 41 distinciones que van a quedar
///         escritas en la cadena para siempre.
///
/// @dev Esto es lo más parecido a un ensayo del despliegue que se puede hacer
///      sin gastar un peso: se lee el mismo JSON que va a leer `forge script`,
///      se arma el mismo catálogo y se despliega contra la EVM del test. Si
///      algo de `script/catalogo.json` está mal —una junta que no existe, una
///      obra que no encadena, un nombre vacío—, se entera acá y no después.
contract CatalogoTest is Test {
    /// @dev 34 piezas (una por habilidad de `content/skills.yaml`) y 7 obras.
    uint256 internal constant CUANTAS = 41;
    uint256 internal constant PIEZAS = 34;
    uint256 internal constant OBRAS = 7;

    string internal constant SALIDA = "test/salida";

    Desplegar internal script;
    Distinciones.Pieza[] internal catalogo;

    function setUp() public {
        script = new Desplegar();
        Distinciones.Pieza[] memory leido = script.leerCatalogo("script/catalogo.json");
        for (uint256 i; i < leido.length; ++i) {
            catalogo.push(leido[i]);
        }
        if (!vm.isDir(SALIDA)) vm.createDir(SALIDA, true);
    }

    function test_SonLas41() public view {
        assertEq(catalogo.length, CUANTAS, "el catalogo no tiene 41 distinciones");

        uint256 piezas;
        uint256 obras;
        for (uint256 i; i < catalogo.length; ++i) {
            if (catalogo[i].esObra) ++obras;
            else ++piezas;
        }
        assertEq(piezas, PIEZAS, "no son 34 piezas");
        assertEq(obras, OBRAS, "no son 7 obras");
    }

    /// @notice Toda distinción tiene nombre, y cada obra tiene la suya, una sola vez.
    function test_CadaObraTieneUnaSolaDistincionDeObra() public view {
        bool[256] memory vista;
        for (uint256 i; i < catalogo.length; ++i) {
            assertTrue(bytes(catalogo[i].nombre).length > 0, "una distincion sin nombre");
            if (!catalogo[i].esObra) continue;
            assertFalse(vista[catalogo[i].obra], "dos distinciones para la misma obra");
            vista[catalogo[i].obra] = true;
        }
        for (uint8 o = 0; o < OBRAS; ++o) {
            assertTrue(vista[o], "falta la distincion de una obra");
        }
    }

    /// @notice Las juntas del catálogo existen y encadenan.
    /// @dev Es la única propiedad del catálogo que el contrato no puede arreglar
    ///      después: si la junta de abajo de una pieza no es la de arriba de la
    ///      siguiente, la obra queda dibujada con un escalón en el medio y no hay
    ///      forma de corregirlo sin desplegar otro contrato.
    function test_LasJuntasExistenYEncadenan() public view {
        for (uint8 obra = 0; obra < OBRAS; ++obra) {
            uint8 esperada = 0; // el pilar arranca recto
            bool hubo = false;
            uint8 ultima = 0;

            for (uint256 i; i < catalogo.length; ++i) {
                Distinciones.Pieza storage p = catalogo[i];
                if (p.esObra || p.obra != obra) continue;

                assertLt(p.juntaArriba, Pilar.JUNTAS, "junta de arriba inexistente");
                assertLt(p.juntaAbajo, Pilar.JUNTAS, "junta de abajo inexistente");
                assertEq(p.juntaArriba, esperada, "la junta no calza con la pieza de arriba");

                esperada = p.juntaAbajo;
                ultima = p.juntaAbajo;
                hubo = true;
            }

            assertTrue(!hubo || ultima == 0, "el pilar no termina recto");
        }
    }

    /// @notice El catálogo de verdad se despliega, y cada una de las 41 dibuja.
    /// @dev El ensayo completo: si alguna distinción hiciera revertir a
    ///      `tokenURI`, sería un token ilegible para siempre.
    function test_ElCatalogoRealSeDespliegaYTodasDibujan() public {
        Distinciones d = _desplegar();

        uint256 pk = 0xA11CE;
        address firmante = vm.addr(pk);
        address quien = makeAddr("quien");

        for (uint16 i = 0; i < CUANTAS; ++i) {
            uint64 deadline = uint64(vm.getBlockTimestamp() + 1 hours);
            bytes32 digest = _digest(d, quien, i, deadline);
            (uint8 v, bytes32 r, bytes32 s) = vm.sign(pk, digest);
            d.mint(quien, i, deadline, abi.encodePacked(r, s, v));

            string memory svg = d.svgDe(d.idDe(quien, i));
            assertTrue(bytes(svg).length > 0, "una distincion no dibujo nada");
            assertTrue(vm.contains(svg, "</svg>"), "un SVG sin cerrar");
        }

        assertEq(d.firmante(), firmante, "el firmante del ensayo");
    }

    /// @notice Escribe a `test/salida` los dibujos de las distinciones de verdad.
    function test_EscribirLosSVGDelCatalogoReal() public {
        Distinciones d = _desplegar();

        // La pieza 8 de la obra 1 lleva dos arcos; la obra 1 es el pilar más
        // largo del catálogo, con catorce piezas.
        vm.writeFile(string.concat(SALIDA, "/catalogo-pieza.svg"), _dibujo(d, 8));
        vm.writeFile(string.concat(SALIDA, "/catalogo-obra.svg"), _dibujo(d, 35));
    }

    // ------------------------------------------------------------------ ayudas

    function _desplegar() internal returns (Distinciones) {
        Distinciones.Pieza[] memory copia = new Distinciones.Pieza[](catalogo.length);
        for (uint256 i; i < catalogo.length; ++i) {
            copia[i] = catalogo[i];
        }
        return new Distinciones(makeAddr("duenio"), vm.addr(0xA11CE), copia);
    }

    function _dibujo(Distinciones d, uint16 pieza) internal returns (string memory) {
        address quien = makeAddr("quien");
        uint64 deadline = uint64(vm.getBlockTimestamp() + 1 hours);
        (uint8 v, bytes32 r, bytes32 s) = vm.sign(0xA11CE, _digest(d, quien, pieza, deadline));
        d.mint(quien, pieza, deadline, abi.encodePacked(r, s, v));
        return d.svgDe(d.idDe(quien, pieza));
    }

    function _digest(Distinciones d, address to, uint16 pieza, uint64 deadline)
        internal
        view
        returns (bytes32)
    {
        (, string memory name, string memory version, uint256 chainId, address verificador,,) =
            d.eip712Domain();

        bytes32 separador = keccak256(
            abi.encode(
                keccak256(
                    "EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"
                ),
                keccak256(bytes(name)),
                keccak256(bytes(version)),
                chainId,
                verificador
            )
        );
        bytes32 hashStruct = keccak256(
            abi.encode(
                keccak256("Distincion(address to,uint16 pieza,uint64 deadline)"),
                to,
                pieza,
                deadline
            )
        );
        return keccak256(abi.encodePacked(hex"1901", separador, hashStruct));
    }
}
